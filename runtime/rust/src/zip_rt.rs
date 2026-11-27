//! Pure-Rust ZIP archive extractor (STORE + RFC 1951 DEFLATE + CRC32) with
//! Zip-Slip path traversal, symlink, and Zip-Bomb (max 256 MB / 10,000 entries) guards.
//!
//! Replaces `runtime/native/joss_zip_rt.inc` with zero manual C allocations.

use crate::crc32_bytes;
use std::ffi::{c_char, CStr};
use std::fs;
use std::path::{Component, Path, PathBuf};

const MAX_ZIP_ENTRIES: usize = 10_000;
const MAX_TOTAL_UNCOMPRESSED: u64 = 256 * 1024 * 1024;
const MAX_ENTRY_UNCOMPRESSED: usize = 64 * 1024 * 1024;

fn u16_le(b: &[u8], off: usize) -> u16 {
    u16::from_le_bytes([b[off], b[off + 1]])
}

fn u32_le(b: &[u8], off: usize) -> u32 {
    u32::from_le_bytes([b[off], b[off + 1], b[off + 2], b[off + 3]])
}

/// Validates that a ZIP entry path cannot escape `dest_dir` (Zip-Slip guard).
pub fn safe_join_zip_path(dest_dir: &Path, entry_name: &str) -> Option<PathBuf> {
    if entry_name.is_empty() || entry_name.contains('\0') {
        return None;
    }
    if entry_name.starts_with('/') || entry_name.starts_with('\\') {
        return None;
    }
    let bytes = entry_name.as_bytes();
    if bytes.len() >= 2 && bytes[0].is_ascii_alphabetic() && bytes[1] == b':' {
        return None;
    }
    let mut rel = PathBuf::new();
    for comp in Path::new(&entry_name.replace('\\', "/")).components() {
        match comp {
            Component::Normal(seg) => rel.push(seg),
            Component::CurDir => {}
            Component::ParentDir | Component::RootDir | Component::Prefix(_) => return None,
        }
    }
    if rel.as_os_str().is_empty() {
        return None;
    }
    Some(dest_dir.join(rel))
}

/* --- Minimal RFC 1951 DEFLATE (uncompressed + fixed + dynamic Huffman) --- */

struct BitReader<'a> {
    data: &'a [u8],
    pos: usize,
    bit_buf: u32,
    bit_cnt: u32,
}

impl<'a> BitReader<'a> {
    fn new(data: &'a [u8]) -> Self {
        Self {
            data,
            pos: 0,
            bit_buf: 0,
            bit_cnt: 0,
        }
    }

    fn read_bits(&mut self, n: u32) -> Option<u32> {
        while self.bit_cnt < n {
            if self.pos >= self.data.len() {
                return None;
            }
            self.bit_buf |= (self.data[self.pos] as u32) << self.bit_cnt;
            self.pos += 1;
            self.bit_cnt += 8;
        }
        let val = self.bit_buf & ((1u32 << n) - 1);
        self.bit_buf >>= n;
        self.bit_cnt -= n;
        Some(val)
    }

    fn align_byte(&mut self) {
        self.bit_buf = 0;
        self.bit_cnt = 0;
    }

    fn read_u16_aligned(&mut self) -> Option<u16> {
        self.align_byte();
        if self.pos + 2 > self.data.len() {
            return None;
        }
        let v = u16_le(self.data, self.pos);
        self.pos += 2;
        Some(v)
    }
}

struct HuffTable {
    counts: [u16; 16],
    symbols: Vec<u16>,
}

impl HuffTable {
    fn build(lengths: &[u8]) -> Option<Self> {
        let mut counts = [0u16; 16];
        for &l in lengths {
            if l as usize >= 16 {
                return None;
            }
            if l > 0 {
                counts[l as usize] += 1;
            }
        }
        let mut offsets = [0u16; 16];
        let mut total = 0u16;
        for bits in 1..16 {
            offsets[bits] = total;
            total = total.checked_add(counts[bits])?;
        }
        let mut symbols = vec![0u16; total as usize];
        for (sym, &l) in lengths.iter().enumerate() {
            if l > 0 {
                let idx = offsets[l as usize] as usize;
                symbols[idx] = sym as u16;
                offsets[l as usize] += 1;
            }
        }
        Some(Self { counts, symbols })
    }

    fn decode(&self, br: &mut BitReader<'_>) -> Option<u16> {
        let mut code = 0i32;
        let mut first = 0i32;
        let mut index = 0i32;
        for len in 1..16 {
            code |= br.read_bits(1)? as i32;
            let count = self.counts[len] as i32;
            if code - count < first {
                return self.symbols.get((index + (code - first)) as usize).copied();
            }
            index += count;
            first = (first + count) << 1;
            code <<= 1;
        }
        None
    }
}

pub fn inflate_raw(input: &[u8], max_out: usize) -> Option<Vec<u8>> {
    const LEN_BASE: [u16; 29] = [
        3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51, 59, 67, 83, 99, 115,
        131, 163, 195, 227, 258,
    ];
    const LEN_EXTRA: [u8; 29] = [
        0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0,
    ];
    const DIST_BASE: [u16; 30] = [
        1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193, 257, 385, 513, 769, 1025, 1537,
        2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577,
    ];
    const DIST_EXTRA: [u8; 30] = [
        0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12,
        13, 13,
    ];
    const CL_ORDER: [usize; 19] = [
        16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15,
    ];

    let mut br = BitReader::new(input);
    let mut out = Vec::new();
    loop {
        let bfinal = br.read_bits(1)?;
        let btype = br.read_bits(2)?;
        if btype == 0 {
            let len = br.read_u16_aligned()? as usize;
            let nlen = br.read_u16_aligned()?;
            if (len as u16) != !nlen || br.pos + len > br.data.len() || out.len() + len > max_out {
                return None;
            }
            out.extend_from_slice(&br.data[br.pos..br.pos + len]);
            br.pos += len;
        } else if btype == 1 || btype == 2 {
            let (lit_tbl, dist_tbl) = if btype == 1 {
                let mut ll = [0u8; 288];
                ll[0..144].fill(8);
                ll[144..256].fill(9);
                ll[256..280].fill(7);
                ll[280..288].fill(8);
                let dl = [5u8; 32];
                (HuffTable::build(&ll)?, HuffTable::build(&dl)?)
            } else {
                let hlit = br.read_bits(5)? as usize + 257;
                let hdist = br.read_bits(5)? as usize + 1;
                let hclen = br.read_bits(4)? as usize + 4;
                let mut cl_lens = [0u8; 19];
                for i in 0..hclen {
                    cl_lens[CL_ORDER[i]] = br.read_bits(3)? as u8;
                }
                let cl_tbl = HuffTable::build(&cl_lens)?;
                let mut lens = Vec::with_capacity(hlit + hdist);
                while lens.len() < hlit + hdist {
                    let sym = cl_tbl.decode(&mut br)?;
                    match sym {
                        0..=15 => lens.push(sym as u8),
                        16 => {
                            let prev = *lens.last()?;
                            let rep = br.read_bits(2)? as usize + 3;
                            lens.resize(lens.len() + rep, prev);
                        }
                        17 => {
                            let rep = br.read_bits(3)? as usize + 3;
                            lens.resize(lens.len() + rep, 0);
                        }
                        18 => {
                            let rep = br.read_bits(7)? as usize + 11;
                            lens.resize(lens.len() + rep, 0);
                        }
                        _ => return None,
                    }
                }
                if lens.len() != hlit + hdist {
                    return None;
                }
                (
                    HuffTable::build(&lens[..hlit])?,
                    HuffTable::build(&lens[hlit..])?,
                )
            };

            loop {
                let sym = lit_tbl.decode(&mut br)?;
                if sym < 256 {
                    if out.len() >= max_out {
                        return None;
                    }
                    out.push(sym as u8);
                } else if sym == 256 {
                    break;
                } else {
                    let lidx = (sym as usize).checked_sub(257)?;
                    if lidx >= LEN_BASE.len() {
                        return None;
                    }
                    let length =
                        LEN_BASE[lidx] as usize + br.read_bits(LEN_EXTRA[lidx] as u32)? as usize;
                    let dsym = dist_tbl.decode(&mut br)? as usize;
                    if dsym >= DIST_BASE.len() {
                        return None;
                    }
                    let dist =
                        DIST_BASE[dsym] as usize + br.read_bits(DIST_EXTRA[dsym] as u32)? as usize;
                    if dist == 0 || dist > out.len() || out.len() + length > max_out {
                        return None;
                    }
                    for _ in 0..length {
                        let b = out[out.len() - dist];
                        out.push(b);
                    }
                }
            }
        } else {
            return None;
        }
        if bfinal != 0 {
            break;
        }
    }
    Some(out)
}

/// Extracts a ZIP archive from `zip_path` into `dest_dir` with full Zip-Slip/Zip-Bomb guards.
pub fn extract_zip(zip_path: &Path, dest_dir: &Path) -> Result<usize, String> {
    let data = fs::read(zip_path).map_err(|e| e.to_string())?;
    if data.len() < 22 {
        return Err("invalid zip: too small".to_owned());
    }
    // Locate EOCD signature 0x06054b50 from end of archive.
    let search_start = data.len().saturating_sub(65_557);
    let mut eocd = None;
    for i in (search_start..=data.len() - 22).rev() {
        if u32_le(&data, i) == 0x0605_4b50 {
            eocd = Some(i);
            break;
        }
    }
    let eocd_off = eocd.ok_or_else(|| "invalid zip: EOCD not found".to_owned())?;
    let total_entries = u16_le(&data, eocd_off + 10) as usize;
    let cd_offset = u32_le(&data, eocd_off + 16) as usize;
    if total_entries > MAX_ZIP_ENTRIES || cd_offset > data.len() {
        return Err("zip guard: too many entries or bad central directory".to_owned());
    }

    fs::create_dir_all(dest_dir).map_err(|e| e.to_string())?;
    let mut pos = cd_offset;
    let mut total_written: u64 = 0;
    let mut extracted = 0usize;

    for _ in 0..total_entries {
        if pos + 46 > data.len() || u32_le(&data, pos) != 0x0201_4b50 {
            return Err("invalid zip central directory header".to_owned());
        }
        let method = u16_le(&data, pos + 10);
        let expected_crc = u32_le(&data, pos + 16);
        let comp_size = u32_le(&data, pos + 20) as usize;
        let uncomp_size = u32_le(&data, pos + 24) as usize;
        let name_len = u16_le(&data, pos + 28) as usize;
        let extra_len = u16_le(&data, pos + 30) as usize;
        let comment_len = u16_le(&data, pos + 32) as usize;
        let ext_attr = u32_le(&data, pos + 38);
        let lfh_off = u32_le(&data, pos + 42) as usize;

        if pos + 46 + name_len + extra_len + comment_len > data.len() {
            return Err("truncated zip entry".to_owned());
        }
        let name = std::str::from_utf8(&data[pos + 46..pos + 46 + name_len])
            .map_err(|_| "non-utf8 zip entry name".to_owned())?;
        pos += 46 + name_len + extra_len + comment_len;

        // Block Unix symlinks (mode 0120000 in upper 16 bits of external attributes)
        let unix_mode = (ext_attr >> 16) & 0o170000;
        if unix_mode == 0o120000 {
            return Err("zip guard: symlink entry rejected".to_owned());
        }

        if name.ends_with('/') {
            if let Some(dir_path) = safe_join_zip_path(dest_dir, name.trim_end_matches('/')) {
                fs::create_dir_all(dir_path).map_err(|e| e.to_string())?;
            }
            continue;
        }

        let target = safe_join_zip_path(dest_dir, name)
            .ok_or_else(|| format!("zip guard: unsafe path {name}"))?;
        if uncomp_size > MAX_ENTRY_UNCOMPRESSED
            || total_written + (uncomp_size as u64) > MAX_TOTAL_UNCOMPRESSED
        {
            return Err("zip guard: zip-bomb size limit exceeded".to_owned());
        }

        if lfh_off + 30 > data.len() || u32_le(&data, lfh_off) != 0x0403_4b50 {
            return Err("invalid local file header".to_owned());
        }
        let l_name_len = u16_le(&data, lfh_off + 26) as usize;
        let l_extra_len = u16_le(&data, lfh_off + 28) as usize;
        let payload_off = lfh_off + 30 + l_name_len + l_extra_len;
        if payload_off + comp_size > data.len() {
            return Err("truncated zip payload".to_owned());
        }
        let compressed = &data[payload_off..payload_off + comp_size];

        let content = match method {
            0 => compressed.to_vec(),
            8 => inflate_raw(compressed, MAX_ENTRY_UNCOMPRESSED)
                .ok_or_else(|| "deflate decompression failed".to_owned())?,
            _ => return Err(format!("unsupported zip compression method {method}")),
        };
        if crc32_bytes(&content) != expected_crc {
            return Err("zip crc32 mismatch".to_owned());
        }
        if let Some(parent) = target.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
        fs::write(&target, &content).map_err(|e| e.to_string())?;
        total_written += content.len() as u64;
        extracted += 1;
    }
    Ok(extracted)
}

/// # Safety
/// `zip_path` and `dest_dir` must be valid NUL-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_zip_extract(
    zip_path: *const c_char,
    dest_dir: *const c_char,
) -> i64 {
    if zip_path.is_null() || dest_dir.is_null() {
        return 0;
    }
    let (Ok(zp), Ok(dd)) = (
        CStr::from_ptr(zip_path).to_str(),
        CStr::from_ptr(dest_dir).to_str(),
    ) else {
        return 0;
    };
    if extract_zip(Path::new(zp), Path::new(dd)).is_ok() {
        1
    } else {
        0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn zip_slip_paths_are_rejected() {
        let base = Path::new("/tmp/out");
        assert!(safe_join_zip_path(base, "../evil.txt").is_none());
        assert!(safe_join_zip_path(base, "/etc/passwd").is_none());
        assert!(safe_join_zip_path(base, "C:\\Windows\\win.ini").is_none());
        assert!(safe_join_zip_path(base, "sub/ok.txt").is_some());
    }

    #[test]
    fn store_zip_roundtrip_extracts_cleanly() {
        let content = b"hello rust zip!";
        let crc = crc32_bytes(content);
        let name = b"dir/hello.txt";
        let mut zip = Vec::new();
        // Local file header
        zip.extend_from_slice(&0x0403_4b50u32.to_le_bytes());
        zip.extend_from_slice(&20u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes()); // STORE
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&crc.to_le_bytes());
        zip.extend_from_slice(&(content.len() as u32).to_le_bytes());
        zip.extend_from_slice(&(content.len() as u32).to_le_bytes());
        zip.extend_from_slice(&(name.len() as u16).to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(name);
        zip.extend_from_slice(content);
        let cd_off = zip.len() as u32;
        // Central directory header
        zip.extend_from_slice(&0x0201_4b50u32.to_le_bytes());
        zip.extend_from_slice(&20u16.to_le_bytes());
        zip.extend_from_slice(&20u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&crc.to_le_bytes());
        zip.extend_from_slice(&(content.len() as u32).to_le_bytes());
        zip.extend_from_slice(&(content.len() as u32).to_le_bytes());
        zip.extend_from_slice(&(name.len() as u16).to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u32.to_le_bytes());
        zip.extend_from_slice(&0u32.to_le_bytes());
        zip.extend_from_slice(name);
        let cd_size = (zip.len() as u32) - cd_off;
        // EOCD
        zip.extend_from_slice(&0x0605_4b50u32.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());
        zip.extend_from_slice(&1u16.to_le_bytes());
        zip.extend_from_slice(&1u16.to_le_bytes());
        zip.extend_from_slice(&cd_size.to_le_bytes());
        zip.extend_from_slice(&cd_off.to_le_bytes());
        zip.extend_from_slice(&0u16.to_le_bytes());

        let base = std::env::temp_dir().join(format!("joss-rs-zip-{}", std::process::id()));
        let _ = fs::remove_dir_all(&base);
        fs::create_dir_all(&base).unwrap();
        let zfile = base.join("test.zip");
        let out_dir = base.join("out");
        fs::write(&zfile, &zip).unwrap();

        assert_eq!(extract_zip(&zfile, &out_dir).unwrap(), 1);
        assert_eq!(fs::read(out_dir.join("dir/hello.txt")).unwrap(), content);
        let _ = fs::remove_dir_all(&base);
    }
}
