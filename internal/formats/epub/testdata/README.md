# EPUB test corpus

Drop real `.epub` files here (including deliberately broken ones) to grow the
parser's real-world corpus. `TestCorpusRealEPUBs` picks up every `*.epub` in
this directory and asserts that parsing never panics and always yields a
non-empty title (falling back to the filename for unreadable files).

The directory is intentionally empty in git — the synthetic EPUBs built in
`epub_test.go` already cover the structural cases (EPUB2/EPUB3, Calibre
metadata, cover-resolution paths, and malformed archives). Real files are for
catching quirks those don't anticipate.
