#include "include.h"

// RELIC library uses macros which are not visible to Go code.

// wrapper function for bn_new macro
void _bn_new(bn_t a) {
    bn_new(a)
}

// wrapper function for bn_free macro
void _bn_free(bn_t a) {
    bn_free(a)
}

// wrapper function for ep_new macro
void _ep_new(ep_t a) {
    ep_new(a);
}

// wrapper function for ep_free macro
void _ep_free(ep_t a) {
    ep_free(a)
}


