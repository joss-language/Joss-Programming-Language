//! Memory-safe RFC 7578 `multipart/form-data` parser in pure Rust.
//!
//! Replaces manual pointer arithmetic & `malloc` parsing in `joss_session_rt.inc`.

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MultipartPart {
    pub name: String,
    pub filename: Option<String>,
    pub content_type: String,
    pub data: Vec<u8>,
}

fn find_bytes(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    if needle.is_empty() || needle.len() > haystack.len() {
        return None;
    }
    haystack.windows(needle.len()).position(|w| w == needle)
}

fn extract_quoted_param(header: &str, key: &str) -> Option<String> {
    let needle = format!("{key}=\"");
    let start = header.find(&needle)? + needle.len();
    let rest = &header[start..];
    let end = rest.find('"')?;
    Some(rest[..end].to_owned())
}

pub fn parse_multipart(body: &[u8], boundary: &str) -> Vec<MultipartPart> {
    let mut parts = Vec::new();
    if boundary.is_empty() || body.is_empty() {
        return parts;
    }
    let delim = format!("--{boundary}");
    let delim_bytes = delim.as_bytes();
    let mut pos = match find_bytes(body, delim_bytes) {
        Some(i) => i + delim_bytes.len(),
        None => return parts,
    };

    while pos < body.len() {
        if body[pos..].starts_with(b"--") {
            break;
        }
        if body[pos..].starts_with(b"\r\n") {
            pos += 2;
        } else if body[pos..].starts_with(b"\n") {
            pos += 1;
        }
        let next_rel = match find_bytes(&body[pos..], delim_bytes) {
            Some(r) => r,
            None => break,
        };
        let chunk = &body[pos..pos + next_rel];
        pos += next_rel + delim_bytes.len();

        let (hdr_end, sep_len) = if let Some(i) = find_bytes(chunk, b"\r\n\r\n") {
            (i, 4)
        } else if let Some(i) = find_bytes(chunk, b"\n\n") {
            (i, 2)
        } else {
            continue;
        };
        let hdr_str = String::from_utf8_lossy(&chunk[..hdr_end]);
        let mut pdata = &chunk[hdr_end + sep_len..];
        if pdata.ends_with(b"\r\n") {
            pdata = &pdata[..pdata.len() - 2];
        } else if pdata.ends_with(b"\n") {
            pdata = &pdata[..pdata.len() - 1];
        }

        let mut name = String::new();
        let mut filename = None;
        let mut content_type = "application/octet-stream".to_owned();
        for line in hdr_str.lines() {
            let lower = line.to_ascii_lowercase();
            if lower.starts_with("content-disposition:") {
                if let Some(n) = extract_quoted_param(line, "name") {
                    name = n;
                }
                filename = extract_quoted_param(line, "filename");
            } else if lower.starts_with("content-type:") {
                if let Some((_, v)) = line.split_once(':') {
                    content_type = v.trim().to_owned();
                }
            }
        }
        if !name.is_empty() {
            parts.push(MultipartPart {
                name,
                filename,
                content_type,
                data: pdata.to_vec(),
            });
        }
    }
    parts
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_text_field_and_binary_upload() {
        let raw = concat!(
            "--BOUND\r\n",
            "Content-Disposition: form-data; name=\"title\"\r\n\r\n",
            "Hola Joss\r\n",
            "--BOUND\r\n",
            "Content-Disposition: form-data; name=\"avatar\"; filename=\"pic.png\"\r\n",
            "Content-Type: image/png\r\n\r\n",
            "PNGDATA\r\n",
            "--BOUND--\r\n"
        );
        let parts = parse_multipart(raw.as_bytes(), "BOUND");
        assert_eq!(parts.len(), 2);
        assert_eq!(parts[0].name, "title");
        assert_eq!(parts[0].filename, None);
        assert_eq!(parts[0].data, b"Hola Joss");
        assert_eq!(parts[1].name, "avatar");
        assert_eq!(parts[1].filename.as_deref(), Some("pic.png"));
        assert_eq!(parts[1].content_type, "image/png");
        assert_eq!(parts[1].data, b"PNGDATA");
    }
}
