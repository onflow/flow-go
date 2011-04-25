/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Benchmarks for the multiple precision integer arithmetic functions.
 *
 * @version $Id$
 * @ingroup bench
 */

#include "relic.h"
#include "relic_bench.h"

static void memory(void) {
	bn_t a[BENCH];

	BENCH_SMALL("bn_null", bn_null(a[i]));

	BENCH_SMALL("bn_new", bn_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		bn_free(a[i]);
	}

	BENCH_SMALL("bn_new_size", bn_new_size(a[i], 2 * BN_DIGS));
	for (int i = 0; i < BENCH; i++) {
		bn_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		bn_new(a[i]);
		bn_clean(a[i]);
	}
	BENCH_SMALL("bn_init", bn_init(a[i], BN_DIGS));
	for (int i = 0; i < BENCH; i++) {
		bn_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		bn_new(a[i]);
	}
	BENCH_SMALL("bn_clean", bn_clean(a[i]));
	for (int i = 0; i < BENCH; i++) {
		bn_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		bn_new(a[i]);
	}
	BENCH_SMALL("bn_grow", bn_grow(a[i], 2 * BN_DIGS));
	for (int i = 0; i < BENCH; i++) {
		bn_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		bn_new(a[i]);
		bn_grow(a[i], 2 * BN_DIGS);
	}
	BENCH_SMALL("bn_trim", bn_trim(a[i]));
	for (int i = 0; i < BENCH; i++) {
		bn_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		bn_new(a[i]);
	}
	BENCH_SMALL("bn_free", bn_free(a[i]));

	for (int i = 0; i < BENCH; i++) {
		bn_new_size(a[i], 2 * BN_DIGS);
	}
	BENCH_SMALL("bn_free (size)", bn_free(a[i]));
}

static void util(void) {
	int d, len;
	dig_t digit;
	char str[BN_DIGS * sizeof(dig_t) * 3 + 1];
	unsigned char bin[BN_DIGS * sizeof(dig_t)];
	dig_t raw[BN_DIGS];
	bn_t a, b;

	bn_null(a);
	bn_null(b);

	bn_new(a);
	bn_new(b);

	bn_rand(b, BN_POS, BN_BITS);

	BENCH_BEGIN("bn_copy") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_abs") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_abs(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_neg") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_sign") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_sign(a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_zero") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_zero(b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_is_zero") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_is_even") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_is_even(a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_bits") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_bits(a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_test_bit") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_test_bit(a, BN_BITS / 2));
	}
	BENCH_END;

	BENCH_BEGIN("bn_get_bit") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_get_bit(a, BN_BITS / 2));
	}
	BENCH_END;

	BENCH_BEGIN("bn_set_bit") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_set_bit(a, BN_BITS / 2, 1));
	}
	BENCH_END;

	BENCH_BEGIN("bn_get_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_get_dig(&digit, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_set_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_set_dig(a, 1));
	}
	BENCH_END;

	BENCH_BEGIN("bn_set_2b") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_set_2b(a, BN_BITS / 2));
	}
	BENCH_END;

	BENCH_BEGIN("bn_rand") {
		BENCH_ADD(bn_rand(a, BN_POS, BN_DIGS));
	}
	BENCH_END;

	BENCH_BEGIN("bn_size_str") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_size_str(&d, a, 10));
	}
	BENCH_END;

	BENCH_BEGIN("bn_write_str") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_write_str(str, sizeof(str), a, 10));
	}
	BENCH_END;

	BENCH_BEGIN("bn_read_str") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_read_str(a, str, sizeof(str), 10));
	}
	BENCH_END;

	BENCH_BEGIN("bn_size_bin") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_size_bin(&d, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_write_bin") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_size_bin(&len, a);
		BENCH_ADD(bn_write_bin(bin, len, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_read_bin") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_size_bin(&len, a);
		BENCH_ADD(bn_read_bin(a, bin, len));
	}
	BENCH_END;

	BENCH_BEGIN("bn_size_raw") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_size_raw(&d, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_write_raw") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_size_bin(&len, a);
		BENCH_ADD(bn_write_raw(raw, len, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_read_raw") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_size_raw(&len, a);
		BENCH_ADD(bn_read_raw(a, raw, len));
	}
	BENCH_END;

	BENCH_BEGIN("bn_cmp_abs") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_cmp_abs(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_cmp_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_cmp_dig(a, (dig_t)0));
	}
	BENCH_END;

	BENCH_BEGIN("bn_cmp") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_cmp(b, a));
	}
	BENCH_END;

	bn_free(a);
	bn_free(b);
}

static void arith(void) {
	bn_t a, b, c, d, e;
	dig_t f;

	bn_null(a);
	bn_null(b);
	bn_null(c);
	bn_null(d);
	bn_null(e);

	bn_new(a);
	bn_new(b);
	bn_new(c);
	bn_new(d);
	bn_new(e);

	BENCH_BEGIN("bn_add") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_add_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		bn_get_dig(&f, b);
		BENCH_ADD(bn_add_dig(c, a, f));
	}
	BENCH_END;

	BENCH_BEGIN("bn_sub") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_sub_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		bn_get_dig(&f, b);
		BENCH_ADD(bn_sub_dig(c, a, f));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mul") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mul_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		bn_get_dig(&f, b);
		BENCH_ADD(bn_mul_dig(c, a, f));
	}
	BENCH_END;

#if BN_MUL == BASIC || !defined(STRIP)
	BENCH_BEGIN("bn_mul_basic") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_mul_basic(c, a, b));
	}
	BENCH_END;
#endif

#if BN_MUL == COMBA || !defined(STRIP)
	BENCH_BEGIN("bn_mul_comba") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_mul_comba(c, a, b));
	}
	BENCH_END;
#endif

#if BN_KARAT > 0 || !defined(STRIP)
	BENCH_BEGIN("bn_mul_karat") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_mul_karat(c, a, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_sqr") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_sqr(c, a));
	}
	BENCH_END;

#if BN_SQR == BASIC || !defined(STRIP)
	BENCH_BEGIN("bn_sqr_basic") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_sqr_basic(c, a));
	}
	BENCH_END;
#endif

#if BN_SQR == COMBA || !defined(STRIP)
	BENCH_BEGIN("bn_sqr_comba") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_sqr_comba(c, a));
	}
	BENCH_END;
#endif

#if BN_KARAT > 0 || !defined(STRIP)
	BENCH_BEGIN("bn_sqr_karat") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_sqr_karat(c, a));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_dbl") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_dbl(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_hlv") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_hlv(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_lsh") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_lsh(c, a, BN_BITS / 2 + BN_DIGIT / 2));
	}
	BENCH_END;

	BENCH_BEGIN("bn_rsh") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_rsh(c, a, BN_BITS / 2 + BN_DIGIT / 2));
	}
	BENCH_END;

	BENCH_BEGIN("bn_div") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_div(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_div_rem") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_div_rem(c, d, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_div_dig") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		do {
			bn_rand(b, BN_POS, BN_DIGIT);
		} while (bn_is_zero(b));
		BENCH_ADD(bn_div_dig(c, a, b->dp[0]));
	}
	BENCH_END;

	BENCH_BEGIN("bn_div_rem_dig") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		do {
			bn_rand(b, BN_POS, BN_DIGIT);
		} while (bn_is_zero(b));
		BENCH_ADD(bn_div_rem_dig(c, &f, a, b->dp[0]));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_2b") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_mod_2b(c, a, BN_BITS / 2));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		do {
			bn_rand(b, BN_POS, BN_DIGIT);
		} while (bn_is_zero(b));
		BENCH_ADD(bn_mod_dig(&f, a, b->dp[0]));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod") {
#if BN_MOD == PMERS
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
		bn_mod_pre(d, b);
#else
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod_pre(d, b);
#endif
		BENCH_ADD(bn_mod(c, a, b, d));
	}
	BENCH_END;

#if BN_MOD == BASIC || !defined(STRIP)
	BENCH_BEGIN("bn_mod_basic") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_mod_basic(c, a, b));
	}
	BENCH_END;
#endif

#if BN_MOD == BARRT || !defined(STRIP)
	BENCH_BEGIN("bn_mod_pre_barrt") {
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_mod_pre_barrt(d, b));
	}
	BENCH_END;
#endif

#if BN_MOD == BARRT || !defined(STRIP)
	BENCH_BEGIN("bn_mod_barrt") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		bn_mod_pre_barrt(d, b);
		BENCH_ADD(bn_mod_barrt(c, a, b, d));
	}
	BENCH_END;
#endif

#if BN_MOD == MONTY || !defined(STRIP)
	BENCH_BEGIN("bn_mod_pre_monty") {
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		BENCH_ADD(bn_mod_pre_monty(d, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_monty_conv") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod(a, a, b);
		BENCH_ADD(bn_mod_monty_conv(a, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_monty") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod(a, a, b);
		bn_mod_pre_monty(d, b);
		BENCH_ADD(bn_mod_monty(c, a, b, d));
	}
	BENCH_END;

#if BN_MUL == BASIC || !defined(STRIP)
	BENCH_BEGIN("bn_mod_monty_basic") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod(a, a, b);
		bn_mod_pre_monty(d, b);
		BENCH_ADD(bn_mod_monty_basic(c, a, b, d));
	}
	BENCH_END;
#endif

#if BN_MUL == COMBA || !defined(STRIP)
	BENCH_BEGIN("bn_mod_monty_comba") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod(a, a, b);
		bn_mod_pre_monty(d, b);
		BENCH_ADD(bn_mod_monty_comba(c, a, b, d));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_mod_monty_back") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod(a, a, b);
		bn_mod_pre_monty(d, b);
		BENCH_ADD(bn_mod_monty_back(c, c, b));
	}
	BENCH_END;
#endif

#if BN_MOD == PMERS || !defined(STRIP)
	BENCH_BEGIN("bn_mod_pre_pmers") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
		BENCH_ADD(bn_mod_pre_pmers(d, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_pmers") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
		bn_mod_pre_pmers(d, b);
		BENCH_ADD(bn_mod_pmers(c, a, b, d));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_mxp") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
#if BN_MOD != PMERS
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
#else
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
#endif
		bn_mod(a, a, b);
		BENCH_ADD(bn_mxp(c, a, b, b));
	}
	BENCH_END;

#if BN_MXP == BASIC || !defined(STRIP)
	BENCH_BEGIN("bn_mxp_basic") {
#if BN_MOD != PMERS
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
#else
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
#endif
		bn_mod(a, a, b);
		BENCH_ADD(bn_mxp_basic(c, a, b, b));
	}
	BENCH_END;
#endif

#if BN_MXP == SLIDE || !defined(STRIP)
	BENCH_BEGIN("bn_mxp_slide") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
#if BN_MOD != PMERS
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
#else
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
#endif
		bn_mod(a, a, b);
		BENCH_ADD(bn_mxp_slide(c, a, b, b));
	}
	BENCH_END;
#endif

#if BN_MXP == CONST || !defined(STRIP)
	BENCH_BEGIN("bn_mxp_monty") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
#if BN_MOD != PMERS
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
#else
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
#endif
		bn_mod(a, a, b);
		BENCH_ADD(bn_mxp_monty(c, a, b, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_gcd") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_gcd(c, a, b));
	}
	BENCH_END;

#if BN_GCD == BASIC || !defined(STRIP)
	BENCH_BEGIN("bn_gcd_basic") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_gcd_basic(c, a, b));
	}
	BENCH_END;
#endif

#if BN_GCD == LEHME || !defined(STRIP)
	BENCH_BEGIN("bn_gcd_lehme") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_gcd_lehme(c, a, b));
	}
	BENCH_END;
#endif

#if BN_GCD == STEIN || !defined(STRIP)
	BENCH_BEGIN("bn_gcd_stein") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_gcd_stein(c, a, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_gcd_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_DIGIT);
		bn_get_dig(&f, b);
		BENCH_ADD(bn_gcd_dig(c, a, f));
	}
	BENCH_END;

	BENCH_BEGIN("bn_gcd_ext") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_gcd_ext(c, d, e, a, b));
	}
	BENCH_END;

#if BN_GCD == BASIC || !defined(STRIP)
	BENCH_BEGIN("bn_gcd_ext_basic") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_gcd_ext_basic(c, d, e, a, b));
	}
	BENCH_END;
#endif

#if BN_GCD == LEHME || !defined(STRIP)
	BENCH_BEGIN("bn_gcd_ext_lehme") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_gcd_ext_lehme(c, d, e, a, b));
	}
	BENCH_END;
#endif

#if BN_GCD == STEIN || !defined(STRIP)
	BENCH_BEGIN("bn_gcd_ext_stein") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_gcd_ext_stein(c, d, e, a, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_gcd_ext_dig") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_DIGIT);
		BENCH_ADD(bn_gcd_ext_dig(c, d, e, a, b->dp[0]));
	}
	BENCH_END;

	BENCH_BEGIN("bn_lcm") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_lcm(c, a, b));
	}
	BENCH_END;

	bn_gen_prime(b, BN_BITS);

	BENCH_BEGIN("bn_smb_leg") {
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_smb_leg(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_smb_jac") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		BENCH_ADD(bn_smb_jac(c, a, b));
	}
	BENCH_END;

	BENCH_ONCE("bn_gen_prime", bn_gen_prime(a, BN_BITS));

#if BN_GEN == BASIC || !defined(STRIP)
	BENCH_ONCE("bn_gen_prime_basic", bn_gen_prime_basic(a, BN_BITS));
#endif

#if BN_GEN == SAFEP || !defined(STRIP)
	BENCH_ONCE("bn_gen_prime_safep", bn_gen_prime_safep(a, BN_BITS));
#endif

#if BN_GEN == STRON || !defined(STRIP)
	BENCH_ONCE("bn_gen_prime_stron", bn_gen_prime_stron(a, BN_BITS));
#endif

	BENCH_ONCE("bn_is_prime", bn_is_prime(a));

	BENCH_ONCE("bn_is_prime_basic", bn_is_prime_basic(a));

	BENCH_ONCE("bn_is_prime_rabin", bn_is_prime_rabin(a));

	BENCH_ONCE("bn_is_prime_solov", bn_is_prime_solov(a));

	bn_rand(a, BN_POS, BN_BITS);

	BENCH_ONCE("bn_factor", bn_factor(c, a));

	BENCH_BEGIN("bn_is_factor") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_is_factor(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_rec_win") {
		unsigned char win[BN_BITS + 1];
		int len;
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_rec_win(win, &len, a, 4));
	}
	BENCH_END;

	BENCH_BEGIN("bn_rec_slw") {
		unsigned char win[BN_BITS + 1];
		int len;
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_rec_slw(win, &len, a, 4));
	}
	BENCH_END;

	BENCH_BEGIN("bn_rec_naf") {
		signed char naf[BN_BITS + 1];
		int len;
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_rec_naf(naf, &len, a, 4));
	}
	BENCH_END;

#if defined(WITH_EB) && defined(EB_KBLTZ)
	if (eb_param_set_any_kbltz() == STS_OK) {
		bn_t vm, s0, s1, n;

		bn_null(vm);
		bn_null(s0);
		bn_null(s1);
		bn_null(n);

		TRY {
			bn_new(vm);
			bn_new(s0);
			bn_new(s1);
			bn_new(n);

			eb_curve_get_vm(vm);
			eb_curve_get_s0(s0);
			eb_curve_get_s1(s1);
			BENCH_BEGIN("bn_rec_tnaf") {
				signed char tnaf[FB_BITS + 8];
				int len;
				bn_rand(a, BN_POS, BN_BITS);
				eb_curve_get_ord(n);
				bn_mod(a, a, n);
				if (eb_curve_opt_a() == OPT_ZERO) {
					BENCH_ADD(bn_rec_tnaf(tnaf, &len, a, vm, s0, s1, -1, FB_BITS, 4));
				} else {
					BENCH_ADD(bn_rec_tnaf(tnaf, &len, a, vm, s0, s1, 1, FB_BITS, 4));
				}
			}
			BENCH_END;
		}
		CATCH_ANY {
			THROW(ERR_CAUGHT);
		}
		FINALLY {
			bn_free(vm);
			bn_free(s0);
			bn_free(s1);
			bn_free(n);
		}
	}
#endif

	BENCH_BEGIN("bn_rec_jsf") {
		signed char jsf[10 * BN_BITS];
		int len;
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_rec_jsf(jsf, &len, a, b));
	}
	BENCH_END;

	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
	bn_free(e);
}

int main(void) {
	core_init();
	conf_print();
	util_print_banner("Benchmarks for the BN module:", 0);
	util_print_banner("Utilities:", 1);
	memory();
	util();
	util_print_banner("Arithmetic:", 1);
	arith();

	core_clean();
	return 0;
}
