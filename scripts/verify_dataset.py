#!/usr/bin/env python3
import argparse
import mmap
import struct
from collections import Counter

MAGIC = b"CWLTREC2"
VERSION = 2
HEADER_FMT = "<8sIIIQ8I8s"
HEADER_SIZE = 68
RECORD_METADATA_FMT = "<QQ64sB3x"
RECORD_METADATA_SIZE = 84
RECORD_SIZE = 24660


def popcount8(x: int) -> int:
    return x.bit_count()


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--target", required=True)
    args = ap.parse_args()

    with open(args.target, "rb") as f:
        size = f.seek(0, 2)
        if size < HEADER_SIZE:
            raise SystemExit(f"FAIL: file too small: {size}")
        expected = HEADER_SIZE + ((size - HEADER_SIZE) // RECORD_SIZE) * RECORD_SIZE
        if size != expected:
            raise SystemExit(
                f"FAIL: size {size} is not header 68 + N*24660"
            )
        f.seek(0)
        mm = mmap.mmap(f.fileno(), 0, access=mmap.ACCESS_READ)

        fields = struct.unpack_from(HEADER_FMT, mm, 0)
        magic, version, record_count, record_size, seed, *rest = fields
        class_counts = rest[:8]
        reserved = rest[8]

        if magic != MAGIC:
            raise SystemExit(f"FAIL: magic={magic!r}")
        if version != VERSION:
            raise SystemExit(f"FAIL: version={version}")
        if record_size != RECORD_SIZE:
            raise SystemExit(f"FAIL: record_size={record_size}")
        if any(reserved):
            raise SystemExit("FAIL: header reserved bytes are non-zero")

        physical_count = (size - HEADER_SIZE) // RECORD_SIZE
        if physical_count != record_count:
            raise SystemExit(
                f"FAIL: header record_count={record_count}, physical={physical_count}"
            )

        masks = Counter()
        multi_hot = 0
        for i in range(record_count):
            off = HEADER_SIZE + i * RECORD_SIZE
            tick_before, tick_after, _, mask = struct.unpack_from(
                RECORD_METADATA_FMT, mm, off
            )
            if tick_after != tick_before + 1:
                raise SystemExit(
                    f"FAIL: record {i} tick {tick_before}->{tick_after}"
                )
            masks[mask] += 1
            if popcount8(mask) >= 2:
                multi_hot += 1

        observed = tuple(masks.get(1 << bit, 0) for bit in range(8))
        if observed != class_counts:
            raise SystemExit(
                f"FAIL: header class counts {class_counts} do not match single-bit tally {observed};"
                " note that multi-hot masks intentionally contribute to each active class"
            )

        print(f"PASS magic={magic.decode()} version={version} records={record_count} seed={seed}")
        print(f"record_size={record_size} file_size={size}")
        print("mask_distribution:")
        for mask, n in sorted(masks.items()):
            print(f"  0x{mask:02x}: {n} records ({popcount8(mask)} active bits)")
        print(f"multi_hot_records={multi_hot}")
        print("NOTE: class_counts are expected to count active labels, not mask frequencies.")
        mm.close()

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
