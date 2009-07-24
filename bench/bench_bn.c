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
 * Benchmarks for the multiple precision integer arithmetic functions.
 *
 * @version $Id: bench_bn.c 39 2009-06-03 18:15:56Z dfaranha $
 * @ingroup bench
 */

#include "relic.h"
#include "relic_bench.h"

void bn_new_impl(bn_t *a) {
	bn_new(*a);
}

void bn_new_size_impl(bn_t *a) {
	bn_new_size(*a, 2 * BN_DIGS);
}

void memory1(void) {
	bn_t a[BENCH + 1] = { NULL };
	bn_t *tmpa;

	BENCH_BEGIN("bn_new") {
		tmpa = a;
		BENCH_ADD(bn_new_impl(tmpa++));
		for (int j = 0; j <= BENCH; j++) {
			bn_free(a[j]);
		}
	}
	BENCH_END;

	BENCH_BEGIN("bn_new_size") {
		tmpa = a;
		BENCH_ADD(bn_new_size_impl(tmpa++));
		for (int j = 0; j <= BENCH; j++) {
			bn_free(a[j]);
		}
	}
	BENCH_END;

	BENCH_BEGIN("bn_init") {
		for (int j = 0; j <= BENCH; j++) {
			bn_new((a[j]));
			bn_clean(a[j]);
		}
		tmpa = a;
		BENCH_ADD(bn_init(*(tmpa++), BN_DIGS));
		for (int j = 0; j <= BENCH; j++) {
			bn_free(a[j]);
		}
	}
	BENCH_END;

	BENCH_BEGIN("bn_clean") {
		for (int j = 0; j <= BENCH; j++) {
			bn_new(a[j]);
		}
		tmpa = a;
		BENCH_ADD(bn_clean(*(tmpa++)));
		for (int j = 0; j <= BENCH; j++) {
			bn_free(a[j]);
		}
	}
	BENCH_END;
}

void memory2(void) {
	bn_t a[BENCH + 1] = { NULL };
	bn_t *tmpa;

	BENCH_BEGIN("bn_grow") {
		for (int j = 0; j <= BENCH; j++) {
			bn_new(a[j]);
		}
		tmpa = a;
		BENCH_ADD(bn_grow(*(tmpa++), 2 * BN_DIGS));
		for (int j = 0; j <= BENCH; j++) {
			bn_free(a[j]);
		}
	}
	BENCH_END;

	BENCH_BEGIN("bn_trim") {
		for (int j = 0; j <= BENCH; j++) {
			bn_new(a[j]);
		}
		tmpa = a;
		BENCH_ADD(bn_trim(*(tmpa++)));
		for (int j = 0; j <= BENCH; j++) {
			bn_free(a[j]);
		}
	}
	BENCH_END;
}

void memory3(void) {
	bn_t a[BENCH + 1] = { NULL };
	bn_t *tmpa;

	BENCH_BEGIN("bn_free") {
		for (int j = 0; j <= BENCH; j++) {
			bn_new(a[j]);
		}
		tmpa = a;
		BENCH_ADD(bn_free(*(tmpa++)));
	}
	BENCH_END;

	BENCH_BEGIN("bn_free (size)") {
		for (int j = 0; j <= BENCH; j++) {
			bn_new_size(a[j], 2 * BN_DIGS);
		}
		tmpa = a;
		BENCH_ADD(bn_free(*(tmpa++)));
	}
	BENCH_END;
}

void util(void) {
	int d, len, sign;
	dig_t digit;
	char str[BN_DIGS * sizeof(dig_t) * 3 + 1];
	unsigned char bin[BN_DIGS * sizeof(dig_t)];
	dig_t raw[BN_DIGS];
	bn_t a = NULL, b = NULL;

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
		BENCH_ADD(bn_write_bin(bin, &len, &sign, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_read_bin") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_size_bin(&len, a);
		BENCH_ADD(bn_read_bin(a, bin, len, BN_POS));
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
		BENCH_ADD(bn_write_raw(raw, &len, &sign, a));
	}
	BENCH_END;

	BENCH_BEGIN("bn_read_raw") {
		bn_rand(a, BN_POS, BN_BITS);
		bn_size_raw(&len, a);
		BENCH_ADD(bn_read_raw(a, raw, len, BN_POS));
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

void arith(void) {
	bn_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL;
	dig_t f;

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

#if BN_MUL == KARAT || !defined(STRIP)
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

#if BN_SQK > 0 || !defined(STRIP)
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

	BENCH_BEGIN("bn_div_norem") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_div_norem(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_div_basic") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_div_basic(c, d, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_div_dig") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		do {
			bn_rand(b, BN_POS, BN_DIGIT);
		} while (bn_is_zero(b));
		BENCH_ADD(bn_div_dig(c, &f, a, b->dp[0]));
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
#if BN_MOD == RADIX
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
		bn_mod_setup(d, b);
#else
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod_setup(d, b);
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
	BENCH_BEGIN("bn_mod_barrt_setup") {
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_mod_barrt_setup(d, b));
	}
	BENCH_END;
#endif

#if BN_MOD == BARRT || !defined(STRIP)
	BENCH_BEGIN("bn_mod_barrt") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		bn_mod_barrt_setup(d, b);
		BENCH_ADD(bn_mod_barrt(c, a, b, d));
	}
	BENCH_END;
#endif

#if BN_MOD == MONTY || !defined(STRIP)
	BENCH_BEGIN("bn_mod_monty_setup") {
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		BENCH_ADD(bn_mod_monty_setup(d, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_monty_conv") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod_basic(a, a, b);
		BENCH_ADD(bn_mod_monty_conv(a, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_monty") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
		bn_mod_basic(a, a, b);
		bn_mod_monty_setup(d, b);
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
		bn_mod_basic(a, a, b);
		bn_mod_monty_setup(d, b);
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
		bn_mod_basic(a, a, b);
		bn_mod_monty_setup(d, b);
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
		bn_mod_basic(a, a, b);
		bn_mod_monty_setup(d, b);
		BENCH_ADD(bn_mod_monty_back(c, c, b));
	}
	BENCH_END;
#endif

#if BN_MOD == RADIX || !defined(STRIP)
	BENCH_BEGIN("bn_mod_radix_check") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
		BENCH_ADD(bn_mod_radix_check(b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_radix_setup") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
		BENCH_ADD(bn_mod_radix_setup(d, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mod_radix") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
		bn_mod_radix_setup(d, b);
		BENCH_ADD(bn_mod_radix(c, a, b, d));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_rec_win") {
		unsigned char win[BN_BITS];
		int len;
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_rec_win(win, &len, a, 4));
	}
	BENCH_END;

	BENCH_BEGIN("bn_rec_naf") {
		signed char naf[BN_BITS];
		int len;
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_rec_naf(naf, &len, a, 4));
	}
	BENCH_END;

#if defined(WITH_EB) && defined(EB_STAND) && defined(EB_KBLTZ)

#if FB_POLYN == 163
	eb_param_set(NIST_K163);
#endif
#if FB_POLYN == 233
	eb_param_set(NIST_K233);
#endif
#if FB_POLYN == 283
	eb_param_set(NIST_K283);
#endif
#if FB_POLYN == 409
	eb_param_set(NIST_K409);
#endif
#if FB_POLYN == 571
	eb_param_set(NIST_K571);
#endif

	BENCH_BEGIN("bn_rec_tnaf") {
		signed char tnaf[FB_BITS + 8];
		int len;
		bn_rand(a, BN_POS, BN_BITS);
		BENCH_ADD(bn_rec_tnaf(tnaf, &len, a, eb_curve_get_vm(),
						eb_curve_get_s0(), eb_curve_get_s1(), 1, FB_BITS, 4));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("bn_rec_jsf") {
		signed char jsf[2 * BN_BITS];
		int len;
		bn_rand(a, BN_POS, BN_BITS);
		bn_rand(b, BN_POS, BN_BITS);
		BENCH_ADD(bn_rec_jsf(jsf, &len, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("bn_mxp") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
#if BN_MOD != RADIX
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
#else
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
#endif
		bn_mod_basic(a, a, b);
		BENCH_ADD(bn_mxp(c, a, b, b));
	}
	BENCH_END;

#if BN_MXP == BASIC || !defined(STRIP)
	BENCH_BEGIN("bn_mxp_basic") {
#if BN_MOD != RADIX
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
#else
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
#endif
		bn_mod_basic(a, a, b);
		BENCH_ADD(bn_mxp_basic(c, a, b, b));
	}
	BENCH_END;
#endif

#if BN_MXP == SLIDE || !defined(STRIP)
	BENCH_BEGIN("bn_mxp_slide") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
#if BN_MOD != RADIX
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
#else
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
#endif
		bn_mod_basic(a, a, b);
		BENCH_ADD(bn_mxp_slide(c, a, b, b));
	}
	BENCH_END;
#endif

#if BN_MXP == CONST || !defined(STRIP)
	BENCH_BEGIN("bn_mxp_const") {
		bn_rand(a, BN_POS, 2 * BN_BITS - BN_DIGIT / 2);
		bn_rand(b, BN_POS, BN_BITS);
#if BN_MOD != RADIX
		if (bn_is_even(b)) {
			bn_add_dig(b, b, 1);
		}
#else
		bn_set_2b(b, BN_BITS);
		bn_rand(c, BN_POS, BN_DIGIT);
		bn_sub(b, b, c);
#endif
		bn_mod_basic(a, a, b);
		BENCH_ADD(bn_mxp_const(c, a, b, b));
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


#undef BENCH
#define BENCH 1

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

	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
}

int main(void) {
	core_init();
	conf_print();

	util_print("\n--- Memory-management:\n\n");
	memory1();
	memory2();
	memory3();
	util_print("\n--- Utilities:\n\n");
	util();
	util_print("\n--- Arithmetic:\n\n");
	arith();

	core_clean();
	return 0;
}
