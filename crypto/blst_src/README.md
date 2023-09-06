All files in this folder contain source files copied from the BLST repo https://github.com/supranational/blst
specifically from the commit <92c12ac58095de04e776cec5ef5ce5bdf242b693>. 

 Copyright Supranational LLC
 Licensed under the Apache License, Version 2.0, see LICENSE for details.
 SPDX-License-Identifier: Apache-2.0

While BLST exports multiple functions and tools, the implementation in Flow crypto requires access to low level functions. Some of these tools are not exported by BLST, others would need to be used without paying for the cgo cost, and therefore without using the Go bindings in BLST. 

The folder contains:
- BLST LICENSE file
- all `<blst>/src/*.c` and `<blst>/src/*.h` files (C source files) but `server.c`.
- `server.c` is replaced by `blst_src.c` (which lists only the files needed by Flow crypto).
- all `<blst>/build`   (assembly generated files).
- `<blst>/bindings/blst.h`  (headers of external functions).
- `<blst>/bindings/blst_aux.h` (headers of external aux functions).
- this `README` file.

To upgrade the BLST version:
- [ ] delete all files in this folder (`./blst_src`) but `blst_src.c` and `README.md`.
- [ ] open BLST repository on the new version.
- [ ] copy all `.c` and `.h` files from `<blst>/src/` into this folder.
- [ ] delete `server.c` from this folder.
- [ ] update `blst_src.c` if needed.
- [ ] copy the folder `<blst>/build/` into this folder.
- [ ] move `./blst_src/build/assembly.S` to `./blst_src/build/blst_assembly.S`.
- [ ] copy `<blst>/bindings/blst.h` and `<blst>/bindings/blst_aux.h` into this folder.
- [ ] check that C flags in `./bls12381_utils.go` still match the C flags in `<blst>/bindings/go/blst.go`.
- [ ] solve all breaking changes that may occur.
- [ ] update the commit version on this `README`.

Remember that Flow crypto is using non exported internal functions from BLST. Checking for interfaces breaking changes in BLST should made along with auditing changes between the old and new versions. This includes checking logical changes and assumptions beyond interfaces, and assessing their security and performance impact on protocols implemented in Flow crypto. 