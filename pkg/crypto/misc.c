#include "relic_include.h"

// DEBUG related functions
void _fp_print(fp_st* a) {
    char* str = malloc(sizeof(char) * fp_size_str(*a, 16));
    fp_write_str(str, 100, *a, 16);
    printf("%s\n", str);
}

void _bn_print(bn_st* a) {
    char* str = malloc(sizeof(char) * bn_size_str(a, 16));
    bn_write_str(str, 100, a, 16);
    printf("%s\n", str);
}

// Ignoring seed for now and relying on relic entropy
bn_st* bn_randZr(char* seed) {
    bn_st r; 
    bn_new(&r);
    g2_get_ord(&r);

    bn_st* x = (bn_st*) malloc(sizeof(bn_st));
    if (!x) return NULL;

    bn_rand_mod(x,&r);
    return x;
}