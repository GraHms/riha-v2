#!/usr/bin/env python3
"""Builds the table RIHA uses to name a customer's network from their IP address.

Source: https://iptoasn.com/data/ip2asn-v4.tsv.gz, public domain (PDDL v1.0).
Usage:  python3 -I build-ipasn.py ip2asn-v4.tsv.gz ../../backend/internal/adapters/netinfo/ipasn.bin.gz

Format (all big endian), gzipped:
  "IPASN1"
  u32 networks; per network: u32 asn, 2 bytes country, u8 name length, name (utf-8)
  u32 ranges;   per range:   u32 first ip, u32 last ip, u32 network index
"""
import gzip, ipaddress, struct, sys

src, dst = sys.argv[1], sys.argv[2]
nets, index, ranges = [], {}, []
with gzip.open(src, "rt", encoding="utf-8", errors="replace") as f:
    for line in f:
        a, b, asn, cc, name = line.rstrip("\n").split("\t")
        asn = int(asn)
        if asn == 0:
            continue
        # "VODACOM - Vodacom Mocambique S.A" style names repeat themselves.
        parts = [p.strip() for p in name.split(" - ")]
        name = parts[-1] if len(parts) > 1 and parts[-1] else name.strip()
        name = name.encode("utf-8")[:80].decode("utf-8", "ignore")
        cc = (cc if len(cc) == 2 else "--").upper()
        key = (asn, cc, name)
        if key not in index:
            index[key] = len(nets)
            nets.append(key)
        ranges.append((int(ipaddress.IPv4Address(a)), int(ipaddress.IPv4Address(b)), index[key]))
ranges.sort()
out = bytearray(b"IPASN1")
out += struct.pack(">I", len(nets))
for asn, cc, name in nets:
    nb = name.encode("utf-8")
    out += struct.pack(">I", asn) + cc.encode("ascii") + struct.pack(">B", len(nb)) + nb
out += struct.pack(">I", len(ranges))
for a, b, i in ranges:
    out += struct.pack(">III", a, b, i)
with gzip.open(dst, "wb", compresslevel=9) as f:
    f.write(bytes(out))
print(f"{len(nets)} networks, {len(ranges)} ranges, {len(out)>>10} KB raw")
