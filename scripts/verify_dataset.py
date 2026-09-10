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
        payload = size - HEADER_SIZE
        if payload % RECORD_SIZE:
            raise SystemExit(f"FAIL: size {size} is not header 68 + N*24660")
        f.seek(0)
        with mmap.mmap(f.fileno(), 0, access=mmap.ACCESS_READ) as mm:
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

            physical_count = payload // RECORD_SIZE
            if physical_count != record_count:
                raise SystemExit(
                    f"FAIL: header record_count={record_count}, physical={physical_count}"
                )

            masks = Counter()
            active_counts = [0] * 8
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
                if mask == 0:
                    raise SystemExit(f"FAIL: record {i} has zero consequence mask")
                masks[mask] += 1
                bits = popcount8(mask)
                if bits >= 2:
                    multi_hot += 1
                for bit in range(8):
                    if mask & (1 << bit):
                        active_counts[bit] += 1

            if tuple(active_counts) != tuple(class_counts):
                raise SystemExit(
                    f"FAIL: header class counts {tuple(class_counts)} do not match active-label tally {tuple(active_counts)}"
                )

            print(f"PASS magic={magic.decode()} version={version} records={record_count} seed={seed}")
            print(f"record_size={record_size} file_size={size}")
            print("mask_distribution:")
            for mask, n in sorted(masks.items()):
                print(f"  0x{mask:02x}: {n} records ({popcount8(mask)} active bits)")
            print(f"multi_hot_records={multi_hot}")
            print(f"active_class_counts={tuple(active_counts)}")
            if multi_hot == 0:
                raise SystemExit("FAIL: no transition contains >=2 active consequence bits")
            print("PASS multi-hot gate: at least one transition contains >=2 active bits")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
