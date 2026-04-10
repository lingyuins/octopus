package static

import "io/fs"

// StaticFS is nil in source checkouts until frontend assets are bundled into the binary by the release pipeline.
var StaticFS fs.FS
