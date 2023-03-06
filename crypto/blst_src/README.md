All files in this folder contain source files copied from the BLST repo https://github.com/supranational/blst
specifically from the commit <92c12ac58095de04e776cec5ef5ce5bdf242b693>. 

 Copyright Supranational LLC
 Licensed under the Apache License, Version 2.0, see LICENSE for details.
 SPDX-License-Identifier: Apache-2.0

While BLST exports multiple functions and tools, the implementation in Flow crypto requires access to low level functions. Some of these tools are not exported by BLST, others would need to be used without paying for the cgo cost, and therefore without using the Go bindings in BLST. 

The folder contains:
- BLST LICENSE file
- all <blst>/src/*.c and <blst>/src/*.h files (C source files)
- all <blst>/build   (assembly generated files)
- <blst>/bindings/blst.h  (headers of external functions)
- <blst>/bindings/blst_aux.h (headers of external aux functions)

TODO: add steps for upgrading the BLST version