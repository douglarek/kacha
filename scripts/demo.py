#!/usr/bin/env python3
"""Turns the frames of TestRecordDemo into docs/demo.gif.

    KACHA_DEMO_DIR=/tmp/frames go test ./internal/overlay -run TestRecordDemo
    python3 scripts/demo.py /tmp/frames docs/demo.gif
"""
import sys
from pathlib import Path

from PIL import Image, ImageChops

frames_dir = Path(sys.argv[1] if len(sys.argv) > 1 else "/tmp/frames")
out = Path(sys.argv[2] if len(sys.argv) > 2 else "docs/demo.gif")
SIZE = (900, 576)
STEP_MS = 50

frames = [Image.open(p).convert("RGB") for p in sorted(frames_dir.glob("*.png"))]
merged, last = [], None
for im in frames:
    if last is not None and ImageChops.difference(im, last[0]).getbbox() is None:
        last[1] += STEP_MS
        continue
    last = [im, STEP_MS]
    merged.append(last)

pictures = [im.resize(SIZE, Image.LANCZOS) for im, _ in merged]
delays = [d for _, d in merged]
# disposal=1 leaves each frame in place, so that the next one stores only the
# pixels that changed: the same pictures, a quarter of the size, where
# disposal=2 restores the background under every frame.
pictures[0].save(out, save_all=True, append_images=pictures[1:], duration=delays,
                 loop=0, optimize=True, disposal=1)
print(f"{out}: {len(pictures)} frames, {sum(delays) / 1000:.1f}s, {out.stat().st_size / 1024 / 1024:.2f} MB")
