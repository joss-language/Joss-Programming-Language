using System;
using System.Diagnostics;

class Program {
    static int CountPrimes(int limit) {
        if (limit < 2) return 0;
        byte[] sieve = new byte[limit + 1];
        Array.Fill(sieve, (byte)1);
        sieve[0] = 0;
        sieve[1] = 0;

        for (int p = 2; p * p <= limit; p++) {
            if (sieve[p] == 1) {
                for (int i = p * p; i <= limit; i += p) {
                    sieve[i] = 0;
                }
            }
        }

        int count = 0;
        for (int i = 2; i <= limit; i++) {
            if (sieve[i] == 1) count++;
        }
        return count;
    }

    static void Main(string[] args) {
        int limit = args.Length > 0 && int.TryParse(args[0], out int l) ? l : 1000000;
        var sw = Stopwatch.StartNew();
        int count = CountPrimes(limit);
        sw.Stop();
        double ms = sw.Elapsed.TotalMilliseconds;
        Console.WriteLine($"LIMIT={limit} COUNT={count} TIME_MS={ms:F3}");
    }
}
