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