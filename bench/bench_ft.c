/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
 * Benchmarks for the ternary field module.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

static void memory(void) {
	ft_t a[BENCH];

	BENCH_SMALL("ft_null", ft_null(a[i]));

	BENCH_SMALL("ft_new", ft_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		ft_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		ft_new(a[i]);
	}
	BENCH_SMALL("ft_free", ft_free(a[i]));

	(void)a;
}

static void util(void) {
	int d;
	char str[1000];

	ft_t a, b;
	ft_new(a);
	ft_new(b);

	BENCH_BEGIN("ft_copy") {
		ft_rand(a);
		BENCH_ADD(ft_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_neg") {
		ft_rand(a);
		BENCH_ADD(ft_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_zero") {
		ft_rand(a);
		BENCH_ADD(ft_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_is_zero") {
		ft_rand(a);
		BENCH_ADD(ft_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_test_bit") {
		ft_rand(a);
		BENCH_ADD(ft_test_bit(a, FT_DIGIT / 2));
	}
	BENCH_END;

	BENCH_BEGIN("ft_get_bit") {
		ft_rand(a);
		BENCH_ADD(ft_test_bit(a, FT_DIGIT / 2));
	}
	BENCH_END;

	BENCH_BEGIN("ft_set_bit") {
		ft_rand(a);
		BENCH_ADD(ft_set_bit(a, FT_DIGIT / 2, 1));
	}
	BENCH_END;

	BENCH_BEGIN("ft_set_dig") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_set_dig(a, b[0]));
	}
	BENCH_END;

	BENCH_BEGIN("ft_bits") {
		ft_rand(a);
		BENCH_ADD(ft_bits(a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_rand") {
		BENCH_ADD(ft_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_size") {
		ft_rand(a);
		BENCH_ADD(ft_size(&d, a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_write") {
		ft_rand(a);
		BENCH_ADD(ft_write(str, sizeof(str), a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_read") {
		ft_rand(a);
		BENCH_ADD(ft_read(a, str, sizeof(str)));
	}
	BENCH_END;

	BENCH_BEGIN("ft_cmp_dig") {
		ft_rand(a);
		BENCH_ADD(ft_cmp_dig(a, (dig_t)0));
	}
	BENCH_END;

	BENCH_BEGIN("ft_cmp") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_cmp(b, a));
	}
	BENCH_END;

	ft_free(a);
	ft_free(b);
}

static void arith(void) {
	ft_t a, b, c;
	dv_t e;
	int bits;

	ft_new(a);
	ft_new(b);
	ft_new(c);
	dv_new(e);

	BENCH_BEGIN("ft_add") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("ft_add_dig") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_add_dig(c, a, b[0]));
	}
	BENCH_END;

	BENCH_BEGIN("ft_sub") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("ft_sub_dig") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_sub_dig(c, a, b[0]));
	}
	BENCH_END;

	BENCH_BEGIN("ft_poly_add") {
		ft_rand(a);
		BENCH_ADD(ft_poly_add(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_poly_sub") {
		ft_rand(a);
		BENCH_ADD(ft_poly_sub(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("ft_mul") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_mul(c, a, b));
	}
	BENCH_END;

//	BENCH_BEGIN("ft_mul_dig") {
//		ft_rand(a);
//		ft_rand(b);
//		BENCH_ADD(ft_mul_dig(c, a, b[0]));
//	}
//	BENCH_END;
//
#if FT_MUL == BASIC || !defined(STRIP)
	BENCH_BEGIN("ft_mul_basic") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_mul_basic(c, a, b));
	}
	BENCH_END;
#endif

#if FT_MUL == INTEG || !defined(STRIP)
	BENCH_BEGIN("ft_mul_integ") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_mul_integ(c, a, b));
	}
	BENCH_END;
#endif

#if FT_MUL == LODAH || !defined(STRIP)
	BENCH_BEGIN("ft_mul_lodah") {
		ft_rand(a);
		ft_rand(b);
		BENCH_ADD(ft_mul_lodah(c, a, b));
	}
	BENCH_END;
#endif
//
//#if FT_KARAT > 0 || !defined(STRIP)
//	BENCH_BEGIN("ft_mul_karat") {
//		ft_rand(a);
//		ft_rand(b);
//		BENCH_ADD(ft_mul_karat(c, a, b));
//	}
//	BENCH_END;
//#endif

	BENCH_BEGIN("ft_cub") {
		ft_rand(a);
		BENCH_ADD(ft_cub(c, a));
	}
	BENCH_END;

#if FT_CUB == BASIC || !defined(STRIP)
	BENCH_BEGIN("ft_cub_basic") {
		ft_rand(a);
		BENCH_ADD(ft_cub_basic(c, a));
	}
	BENCH_END;
#endif

#if FT_CUB == INTEG || !defined(STRIP)
	BENCH_BEGIN("ft_cub_integ") {
		ft_rand(a);
		BENCH_ADD(ft_cub_integ(c, a));
	}
	BENCH_END;
#endif

#if FT_CUB == TABLE || !defined(STRIP)
	BENCH_BEGIN("ft_cub_table") {
		ft_rand(a);
		BENCH_ADD(ft_cub_table(c, a));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ft_lsh") {
		ft_rand(a);
		a[FT_DIGS - 1] = 0;
		bits = a[0] & MASK(FT_DIG_LOG);
		BENCH_ADD(ft_lsh(c, a, bits));
	}
	BENCH_END;

	BENCH_BEGIN("ft_rsh") {
		ft_rand(a);
		a[FT_DIGS - 1] = 0;
		bits = a[0] & MASK(FT_DIG_LOG);
		BENCH_ADD(ft_rsh(c, a, bits));

	}
	BENCH_END;

	BENCH_BEGIN("ft_rdc_mul") {
		ft_rand(e);
		ft_rand(e + FT_DIGS);
		BENCH_ADD(ft_rdc_mul(c, e));
	}
	BENCH_END;

#if FT_RDC == BASIC || !defined(STRIP)
	BENCH_BEGIN("ft_rdc_mul_basic") {
		ft_rand(e);
		ft_rand(e + FT_DIGS);
		BENCH_ADD(ft_rdc_mul_basic(c, e));
	}
	BENCH_END;
#endif

#if FT_RDC == QUICK || !defined(STRIP)
	BENCH_BEGIN("ft_rdc_mul_quick") {
		ft_rand(e);
		ft_rand(e + FT_DIGS);
		BENCH_ADD(ft_rdc_mul_quick(c, e));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ft_rdc_cub") {
		ft_rand(e);
		ft_rand(e + FT_DIGS);
		ft_rand(e + 2 * FT_DIGS);
		BENCH_ADD(ft_rdc_cub(c, e));
	}
	BENCH_END;

#if FT_RDC == BASIC || !defined(STRIP)
	BENCH_BEGIN("ft_rdc_cub_basic") {
		ft_rand(e);
		ft_rand(e + FT_DIGS);
		ft_rand(e + 2 * FT_DIGS);
		BENCH_ADD(ft_rdc_cub_basic(c, e));
	}
	BENCH_END;
#endif

#if FT_RDC == QUICK || !defined(STRIP)
	BENCH_BEGIN("ft_rdc_cub_quick") {
		ft_rand(e);
		ft_rand(e + FT_DIGS);
		ft_rand(e + 2 * FT_DIGS);
		BENCH_ADD(ft_rdc_cub_quick(c, e));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ft_crt") {
		ft_rand(a);
		ft_cub(e, a);
		BENCH_ADD(ft_crt(c, e));
	}
	BENCH_END;

#if FT_CRT == BASIC || !defined(STRIP)
	BENCH_BEGIN("ft_crt_basic") {
		ft_rand(a);
		ft_cub(e, a);
		BENCH_ADD(ft_crt_basic(c, e));
	}
	BENCH_END;
#endif

#if FT_CRT == QUICK || !defined(STRIP)
	BENCH_BEGIN("ft_crt_quick") {
		ft_rand(a);
		ft_cub(e, a);
		BENCH_ADD(ft_crt_quick(c, e));
	}
	BENCH_END;
#endif
//
//	BENCH_BEGIN("ft_trc") {
//		ft_rand(a);
//		BENCH_ADD(ft_trc(c, a));
//	}
//	BENCH_END;
//
//#if FT_TRC == BASIC || !defined(STRIP)
//	BENCH_BEGIN("ft_trc_basic") {
//		ft_rand(a);
//		BENCH_ADD(ft_trc_basic(c, a));
//	}
//	BENCH_END;
//#endif
//
//#if FT_TRC == QUICK || !defined(STRIP)
//	BENCH_BEGIN("ft_trc_quick") {
//		ft_rand(a);
//		BENCH_ADD(ft_trc_quick(c, a));
//	}
//	BENCH_END;
//#endif
//
//	BENCH_BEGIN("ft_slv") {
//		ft_rand(a);
//		BENCH_ADD(ft_slv(c, a));
//	}
//	BENCH_END;
//
//#if FT_SLV == BASIC || !defined(STRIP)
//	BENCH_BEGIN("ft_slv_basic") {
//		ft_rand(a);
//		BENCH_ADD(ft_slv_basic(c, a));
//	}
//	BENCH_END;
//#endif
//
//#if FT_SLV == QUICK || !defined(STRIP)
//	BENCH_BEGIN("ft_slv_quick") {
//		ft_rand(a);
//		BENCH_ADD(ft_slv_quick(c, a));
//	}
//	BENCH_END;
//#endif
//
//	BENCH_BEGIN("ft_inv") {
//		ft_rand(a);
//		BENCH_ADD(ft_inv(c, a));
//	}
//	BENCH_END;
//
//#if FT_INV == BASIC || !defined(STRIP)
//	BENCH_BEGIN("ft_inv_basic") {
//		ft_rand(a);
//		BENCH_ADD(ft_inv_basic(c, a));
//	}
//	BENCH_END;
//#endif
//
//#if FT_INV == BINAR || !defined(STRIP)
//	BENCH_BEGIN("ft_inv_binar") {
//		ft_rand(a);
//		BENCH_ADD(ft_inv_binar(c, a));
//	}
//	BENCH_END;
//#endif
//
//#if FT_INV == EXGCD || !defined(STRIP)
//	BENCH_BEGIN("ft_inv_exgcd") {
//		ft_rand(a);
//		BENCH_ADD(ft_inv_exgcd(c, a));
//	}
//	BENCH_END;
//#endif
//
//#if FT_INV == ALMOS || !defined(STRIP)
//	BENCH_BEGIN("ft_inv_almos") {
//		ft_rand(a);
//		BENCH_ADD(ft_inv_almos(c, a));
//	}
//	BENCH_END;
//#endif
//
//	BENCH_BEGIN("ft_exp_2b") {
//		ft_rand(a);
//		BENCH_ADD(ft_exp_2b(c, a, 5));
//	}
//	BENCH_END;

	ft_free(a);
	ft_free(b);
	ft_free(c);
	dv_free(e);
}

int main(void) {
	core_init();
	conf_print();
	util_banner("Benchmarks for the FT module:", 0);

	ft_param_set_any();
	ft_param_print();
	util_banner("Utilities:\n", 0);
	memory();
	util();
	util_banner("Arithmetic:\n", 0);
	arith();

	core_clean();
	return 0;
}
