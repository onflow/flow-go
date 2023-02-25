// +build relic

// extra tools to use BLST low level that are needed by the Flow crypto library

#include "blst_include.h"
#include "bls12381_utils.h"

// internal type of BLST `pow256` uses bytes little endian.
// input is bytes big endian as used by Flow crypto lib external scalars.
void pow256_from_be_bytes(pow256 ret, const unsigned char a[Fr_BYTES])
{
    unsigned char* b = (unsigned char*)a + Fr_BYTES - 1;
    if ((uptr_t)ret == (uptr_t)a) { // swap in place
        for (int i=0; i<Fr_BYTES/2; i++) {
            unsigned char tmp = *ret;
            *(ret++) = *b;
            *(b--) = tmp;
        }
        return;
    }
    for (int i=0; i<Fr_BYTES; i++) {
        *(ret++) = *(b--);
    }
}