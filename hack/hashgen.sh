#!/bin/sh

for f in bin/mixctl*; do shasum -a 256 $f > $f.sha256; done
