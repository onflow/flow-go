/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
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
 * Benchmarks for the binary field module.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_fb_low.h"
#include "relic_bench.h"

static void fb_new_impl(fb_t *a) {
	fb_new(*a);
}

static void memory(void) {
	fb_t a[BENCH + 1] = { NULL };
	fb_t *tmpa;

	BENCH_BEGIN("fb_new") {
		tmpa = a;
		BENCH_ADD(fb_new_impl(tmpa++));
		for (int j = 0; j <= BENCH; j++) {
			fb_free(a[j]);
		}
	}
	BENCH_END;

	BENCH_BEGIN("fb_free") {
		for (int j = 0; j <= BENCH; j++) {
			fb_new(a[j]);
		}
		tmpa = a;
		BENCH_ADD(fb_free(*(tmpa++)));
	}
	BENCH_END;
}

static void util(void) {
	int d;
	char str[1000];

	fb_t a, b;
	fb_new(a);
	fb_new(b);

	BENCH_BEGIN("fb_copy") {
		fb_rand(a);
		BENCH_ADD(fb_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb_neg") {
		fb_rand(a);
		BENCH_ADD(fb_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb_zero") {
		fb_rand(a);
		BENCH_ADD(fb_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb_is_zero") {
		fb_rand(a);
		BENCH_ADD(fb_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb_test_bit") {
		fb_rand(a);
		BENCH_ADD(fb_test_bit(a, FB_DIGIT / 2));
	}
	BENCH_END;

	BENCH_BEGIN("fb_get_bit") {
		fb_rand(a);
		BENCH_ADD(fb_test_bit(a, FB_DIGIT / 2));
	}
	BENCH_END;

	BENCH_BEGIN("fb_set_bit") {
		fb_rand(a);
		BENCH_ADD(fb_set_bit(a, FB_DIGIT / 2, 1));
	}
	BENCH_END;

	BENCH_BEGIN("fb_bits") {
		fb_rand(a);
		BENCH_ADD(fb_bits(a));
	}
	BENCH_END;

	d = 0;
	BENCH_BEGIN("fb_bits_dig") {
		fb_zero(a);
		d = (d + 1) % FB_DIGIT;
		fb_set_bit(a, d, 1);
		BENCH_ADD(fb_bits_dig(a[0]));
	}
	BENCH_END;

	BENCH_BEGIN("fb_rand") {
		BENCH_ADD(fb_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb_size") {
		fb_rand(a);
		BENCH_ADD(fb_size(&d, a, 16));
	}
	BENCH_END;

	BENCH_BEGIN("fb_write") {
		fb_rand(a);
		BENCH_ADD(fb_write(str, sizeof(str), a, 16));
	}
	BENCH_END;

	BENCH_BEGIN("fb_read") {
		fb_rand(a);
		BENCH_ADD(fb_read(a, str, sizeof(str), 16));
	}
	BENCH_END;

	BENCH_BEGIN("fb_cmp_dig") {
		fb_rand(a);
		BENCH_ADD(fb_cmp_dig(a, (dig_t)0));
	}
	BENCH_END;

	BENCH_BEGIN("fb_cmp") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_cmp(b, a));
	}
	BENCH_END;

	fb_free(a);
	fb_free(b);
}

static void arith(void) {
	fb_t a, b, c;
	dv_t e;

	fb_new(a);
	fb_new(b);
	fb_new(c);
	dv_new(e);
	dv_zero(e, 2 * FB_DIGS);

	BENCH_BEGIN("fb_add") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb_add_dig") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_add_dig(c, a, b[0]));
	}
	BENCH_END;

	BENCH_BEGIN("fb_sub") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb_sub_dig") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_sub_dig(c, a, b[0]));
	}
	BENCH_END;

	BENCH_BEGIN("fb_mul") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb_mul_dig") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_mul_dig(c, a, b[0]));
	}
	BENCH_END;

#if FB_MUL == BASIC || !defined(STRIP)
	BENCH_BEGIN("fb_mul_basic") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_mul_basic(c, a, b));
	}
	BENCH_END;
#endif

#if FB_MUL == INTEG || !defined(STRIP)
	BENCH_BEGIN("fb_mul_integ") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_mul_integ(c, a, b));
	}
	BENCH_END;
#endif

#if FB_MUL == LCOMB || !defined(STRIP)
	BENCH_BEGIN("fb_mul_lcomb") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_mul_lcomb(c, a, b));
	}
	BENCH_END;
#endif

#if FB_MUL == RCOMB || !defined(STRIP)
	BENCH_BEGIN("fb_mul_rcomb") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_mul_rcomb(c, a, b));
	}
	BENCH_END;
#endif

#if FB_MUL == LODAH || !defined(STRIP)
	BENCH_BEGIN("fb_mul_lodah") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_mul_lodah(c, a, b));
	}
	BENCH_END;
#endif

#if FB_KARAT > 0 || !defined(STRIP)
	BENCH_BEGIN("fb_mul_karat") {
		fb_rand(a);
		fb_rand(b);
		BENCH_ADD(fb_mul_karat(c, a, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fb_sqr") {
		fb_rand(a);
		BENCH_ADD(fb_sqr(c, a));
	}
	BENCH_END;

#if FB_SQR == BASIC || !defined(STRIP)
	BENCH_BEGIN("fb_sqr_basic") {
		fb_rand(a);
		BENCH_ADD(fb_sqr_basic(c, a));
	}
	BENCH_END;
#endif

#if FB_SQR == INTEG || !defined(STRIP)
	BENCH_BEGIN("fb_sqr_integ") {
		fb_rand(a);
		BENCH_ADD(fb_sqr_integ(c, a));
	}
	BENCH_END;
#endif

#if FB_SQR == TABLE || !defined(STRIP)
	BENCH_BEGIN("fb_sqr_table") {
		fb_rand(a);
		BENCH_ADD(fb_sqr_table(c, a));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fb_lsh") {
		fb_rand(a);
		a[FB_DIGS - 1] = 0;
		BENCH_ADD(fb_lsh(c, a, FB_DIGIT/2));
	}
	BENCH_END;

	BENCH_BEGIN("fb_rsh") {
		fb_rand(a);
		a[FB_DIGS - 1] = 0;
		BENCH_ADD(fb_rsh(c, a, FB_BITS/2));
	}
	BENCH_END;

	BENCH_BEGIN("fb_rdc") {
		fb_rand(a);
		fb_lsh(e, a, FB_BITS);
		fb_rand(e);
		BENCH_ADD(fb_rdc(c, e));
	}
	BENCH_END;

#if FB_RDC == BASIC || !defined(STRIP)
	BENCH_BEGIN("fb_rdc_basic") {
		fb_rand(a);
		fb_lsh(e, a, FB_BITS);
		fb_rand(e);
		BENCH_ADD(fb_rdc_basic(c, e));
	}
	BENCH_END;
#endif

#if FB_RDC == QUICK || !defined(STRIP)
	BENCH_BEGIN("fb_rdc_quick") {
		fb_rand(a);
		fb_lsh(e, a, FB_BITS);
		fb_rand(e);
		BENCH_ADD(fb_rdc_quick(c, e));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fb_srt") {
		fb_rand(a);
		fb_sqr(e, a);
		BENCH_ADD(fb_srt(c, e));
	}
	BENCH_END;

#if FB_SRT == BASIC || !defined(STRIP)
	BENCH_BEGIN("fb_srt_basic") {
		fb_rand(a);
		fb_sqr(e, a);
		BENCH_ADD(fb_srt_basic(c, e));
	}
	BENCH_END;
#endif

#if FB_SRT == QUICK || !defined(STRIP)
	BENCH_BEGIN("fb_srt_quick") {
		fb_rand(a);
		fb_sqr(e, a);
		BENCH_ADD(fb_srt_quick(c, e));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fb_trc") {
		fb_rand(a);
		BENCH_ADD(fb_trc(c, a));
	}
	BENCH_END;

#if FB_SRT == BASIC || !defined(STRIP)
	BENCH_BEGIN("fb_trc_basic") {
		fb_rand(a);
		BENCH_ADD(fb_trc_basic(c, a));
	}
	BENCH_END;
#endif

#if FB_SRT == QUICK || !defined(STRIP)
	BENCH_BEGIN("fb_trc_quick") {
		fb_rand(a);
		BENCH_ADD(fb_trc_quick(c, a));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fb_inv") {
		fb_rand(a);
		fb_sqr(e, a);
		BENCH_ADD(fb_inv(c, a));
	}
	BENCH_END;

#if FB_INV == BASIC || !defined(STRIP)
	BENCH_BEGIN("fb_inv_basic") {
		fb_rand(a);
		BENCH_ADD(fb_inv_basic(c, a));
	}
	BENCH_END;
#endif

#if FB_INV == EXGCD || !defined(STRIP)
	BENCH_BEGIN("fb_inv_exgcd") {
		fb_rand(a);
		BENCH_ADD(fb_inv_exgcd(c, a));
	}
	BENCH_END;
#endif

#if FB_INV == ALMOS || !defined(STRIP)
	BENCH_BEGIN("fb_inv_almos") {
		fb_rand(a);
		BENCH_ADD(fb_inv_almos(c, a));
	}
	BENCH_END;
#endif

	fb_free(a);
	fb_free(b);
	fb_free(c);
	dv_free(e);
}

int main(void) {
	core_init();
	conf_print();
	util_print_banner("Benchmarks for the FB module:", 0);

	fb_param_set_any();
	fb_param_print();
	util_print_banner("Utilities:\n", 0);
	memory();
	util();
	util_print_banner("Arithmetic:\n", 0);
	arith();

	core_clean();
	return 0;
}
