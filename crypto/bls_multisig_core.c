// +build relic

#include "bls_include.h"


// Computes the sum of the signatures (G1 elements) flattened in an single sigs array
// and store the sum bytes in dest
// The function assmues len is non-zero and sigs is correctly allocated.
int G1_sum_vector(byte* dest, const byte* sigs, const int len) {
    // temp variables
    ep_t acc, sig;        
    ep_new(acc);
    ep_new(sig);

    // acc = sigs[0]
    if (ep_read_bin_compact(acc, sigs, SIGNATURE_LEN) != RLC_OK)
        return INVALID;
    // sum the remaining points
    for (int i=1; i < len; i++) {
        if (ep_read_bin_compact(sig, &sigs[SIGNATURE_LEN*i], SIGNATURE_LEN) != RLC_OK)
            return INVALID;
        ep_add_projc(acc, acc, sig);
    }
    // export the result
    ep_write_bin_compact(dest, acc, SIGNATURE_LEN);

    // free the temp memory
    ep_free(acc);
    ep_free(sig);
    return VALID;
}