/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Benchmarks for the elementary prime field arithmetic functions.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

static void memory(void) {
	fp_t a[BENCH] = { NULL };

	BENCH_SMALL("fb_new", fp_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		fp_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		fp_new(a[i]);
	}
	BENCH_SMALL("fp_free", fp_free(a[i]));

	(void)a;
}

static void util(void) {
	int d;
	char str[1000];

	fp_t a, b;
	fp_new(a);
	fp_new(b);

	BENCH_BEGIN("fp_copy") {
		fp_rand(a);
		BENCH_ADD(fp_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_neg") {
		fp_rand(a);
		BENCH_ADD(fp_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_zero") {
		fp_rand(a);
		BENCH_ADD(fp_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_is_zero") {
		fp_rand(a);
		BENCH_ADD(fp_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_test_bit") {
		fp_rand(a);
		BENCH_ADD(fp_test_bit(a, FP_DIGIT / 2));
	}
	BENCH_END;

	BENCH_BEGIN("fp_get_bit") {
		fp_rand(a);
		BENCH_ADD(fp_test_bit(a, FP_DIGIT / 2));
	}
	BENCH_END;

	BENCH_BEGIN("fp_set_bit") {
		fp_rand(a);
		BENCH_ADD(fp_set_bit(a, FP_DIGIT / 2, 1));
	}
	BENCH_END;

	BENCH_BEGIN("fp_bits") {
		fp_rand(a);
		BENCH_ADD(fp_bits(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_rand") {
		BENCH_ADD(fp_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_size") {
		fp_rand(a);
		BENCH_ADD(fp_size(&d, a, 16));
	}
	BENCH_END;

	BENCH_BEGIN("fp_write") {
		fp_rand(a);
		BENCH_ADD(fp_write(str, sizeof(str), a, 16));
	}
	BENCH_END;

	BENCH_BEGIN("fp_read") {
		fp_rand(a);
		BENCH_ADD(fp_read(a, str, sizeof(str), 16));
	}
	BENCH_END;

	BENCH_BEGIN("fp_cmp_dig") {
		fp_rand(a);
		BENCH_ADD(fp_cmp_dig(a, (dig_t)0));
	}
	BENCH_END;

	BENCH_BEGIN("fp_cmp") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_cmp(b, a));
	}
	BENCH_END;

	fp_free(a);
	fp_free(b);
}

static void arith(void) {
	fp_t a = NULL, b = NULL, c = NULL;
	dv_t d;
	bn_t e = NULL;

	fp_new(a);
	fp_new(b);
	fp_new(c);
	dv_new(d);
	bn_new(e);

	BENCH_BEGIN("fp_add") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp_add_dig") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_add_dig(c, a, b[0]));
	}
	BENCH_END;

	BENCH_BEGIN("fp_sub") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp_sub_dig") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_sub_dig(c, a, b[0]));
	}
	BENCH_END;

	BENCH_BEGIN("fp_mul") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_mul(c, a, b));
	}
	BENCH_END;

#if FP_MUL == BASIC || !defined(STRIP)
	BENCH_BEGIN("fp_mul_basic") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_mul_basic(c, a, b));
	}
	BENCH_END;
#endif

#if FP_MUL == COMBA || !defined(STRIP)
	BENCH_BEGIN("fp_mul_comba") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_mul_comba(c, a, b));
	}
	BENCH_END;
#endif

#if FP_KARAT > 0 || !defined(STRIP)
	BENCH_BEGIN("fp_mul_karat") {
		fp_rand(a);
		fp_rand(b);
		BENCH_ADD(fp_mul_karat(c, a, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fp_sqr") {
		fp_rand(a);
		BENCH_ADD(fp_sqr(c, a));
	}
	BENCH_END;

#if FP_SQR == BASIC || !defined(STRIP)
	BENCH_BEGIN("fp_sqr_basic") {
		fp_rand(a);
		BENCH_ADD(fp_sqr_basic(c, a));
	}
	BENCH_END;
#endif

#if FP_SQR == COMBA || !defined(STRIP)
	BENCH_BEGIN("fp_sqr_comba") {
		fp_rand(a);
		BENCH_ADD(fp_sqr_comba(c, a));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fp_dbl") {
		fp_rand(a);
		BENCH_ADD(fp_dbl(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_hlv") {
		fp_rand(a);
		BENCH_ADD(fp_hlv(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_lsh") {
		fp_rand(a);
		a[FP_DIGS - 1] = 0;
		BENCH_ADD(fp_lsh(c, a, FP_DIGIT/2));
	}
	BENCH_END;

	BENCH_BEGIN("fp_rsh") {
		fp_rand(a);
		a[FP_DIGS - 1] = 0;
		BENCH_ADD(fp_rsh(c, a, FP_BITS/2));
	}
	BENCH_END;

	BENCH_BEGIN("fp_inv") {
		fp_rand(a);
		BENCH_ADD(fp_inv(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp_rdc") {
		fp_rand(a);
		fp_lsh(d, a, FP_BITS);
		BENCH_ADD(fp_rdc(c, d));
	}
	BENCH_END;

	BENCH_BEGIN("fp_rdc_monty") {
		fp_rand(a);
		fp_lsh(d, a, FP_BITS);
		BENCH_ADD(fp_rdc_monty(c, d));
	}
	BENCH_END;

#if FP_MUL == BASIC || !defined(STRIP)
	BENCH_BEGIN("fp_rdc_monty_basic") {
		fp_rand(a);
		fp_lsh(d, a, FP_BITS);
		BENCH_ADD(fp_rdc_monty_basic(c, d));
	}
	BENCH_END;
#endif

#if FP_MUL == COMBA || !defined(STRIP)
	BENCH_BEGIN("fp_rdc_monty_comba") {
		fp_rand(a);
		fp_lsh(d, a, FP_BITS);
		BENCH_ADD(fp_rdc_monty_comba(c, d));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fp_prime_conv") {
		bn_rand(e, BN_POS, FP_BITS);
		BENCH_ADD(fp_prime_conv(a, e));
	}
	BENCH_END;

	BENCH_BEGIN("fp_order_conv_dig") {
		bn_rand(e, BN_POS, FP_BITS);
		BENCH_ADD(fp_prime_conv_dig(a, e->dp[0]));
	}
	BENCH_END;

	BENCH_BEGIN("fp_order_back") {
		fp_rand(c);
		BENCH_ADD(fp_prime_back(e, c));
	}
	BENCH_END;

	fp_free(a);
	fp_free(b);
	fp_free(c);
	dv_free(d);
	bn_free(e);
}

int main(void) {
	core_init();
	conf_print();
	util_print_banner("Benchmarks for the FP module:", 0);

	bn_t modulus = NULL;

	bn_new(modulus);
	bn_gen_prime(modulus, FP_BITS);
	fp_prime_set(modulus);

	util_print_banner("Utilities:\n", 0);
	memory();
	util();
	util_print_banner("Arithmetic:\n", 0);
	arith();

	core_clean();
	return 0;
}
