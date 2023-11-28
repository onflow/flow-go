All files in this folder contain source files copied from the BLST repo https://github.com/supranational/blst, 
specifically from the tagged version `v0.3.11`.

 Copyright Supranational LLC
 Licensed under the Apache License, Version 2.0, see LICENSE for details.
 SPDX-License-Identifier: Apache-2.0

While BLST exports multiple functions and tools, the implementation in Flow crypto requires access to low level functions. Some of these tools are not exported by BLST, others would need to be used without paying for the cgo cost, and therefore without using the Go bindings in BLST. 

The folder contains:
- BLST LICENSE file
- all `<blst>/src/*.c` and `<blst>/src/*.h` files (C source files) but `server.c`.
- `server.c` is replaced by `./blst_src.c` (which lists only the files needed by Flow crypto).
- all `<blst>/build`   (assembly generated files).
- this `README` file.

To upgrade the BLST version:
- [ ] audit all BLST updates, with focus on `<blst>/src`: https://github.com/supranational/blst/compare/v0.3.11...<new_version>
- [ ] delete all files in this folder `./blst_src/` but `blst_src.c` and `README.md`.
- [ ] delete all files in `./internal/blst/`.
- [ ] open BLST repository on the new version.
- [ ] copy all `.c` and `.h` files from `<blst>/src/` into `./blst_src/`.
- [ ] delete newly copied `./blst_src/server.c`.
- [ ] copy the folder `<blst>/build/` into this folder `./blst_src`.
- [ ] copy `<blst>/bindings/blst.h`, `<blst>/bindings/blst_aux.h`, and `<blst>/bindings/go/blst.go` into `./internal/blst/.`.
- [ ] check that C flags in `./bls12381_utils.go` still include the C flags in `<blst>/bindings/go/blst.go`.
- [ ] update `./blst_src/blst_src.c` if needed.
- [ ] solve all breaking changes that may occur.
- [ ] update the commit version on this `./blst_src/README`.

Note that Flow crypto is using non exported internal functions from BLST. Checking for interfaces breaking changes in BLST should be done along with auditing changes between the old and new versions. This includes checking logical changes and assumptions beyond interfaces, and assessing their security and performance impact on protocols implemented in Flow crypto.
