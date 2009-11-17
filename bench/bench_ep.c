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
 * Benchmarks for the binary elliptic curve module.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

static void memory(void) {
	ep_t a[BENCH];

	BENCH_SMALL("ep_null", ep_null(a[i]));

	BENCH_SMALL("ep_new", ep_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		ep_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		ep_new(a[i]);
	}
	BENCH_SMALL("ep_free", ep_free(a[i]));

	(void)a;
}

static void util(void) {
	ep_t p, q;

	ep_null(p);
	ep_null(q);

	ep_new(p);
	ep_new(q);

	BENCH_BEGIN("ep_is_infty") {
		ep_rand(p);
		BENCH_ADD(ep_is_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_set_infty") {
		ep_rand(p);
		BENCH_ADD(ep_set_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_copy") {
		ep_rand(p);
		ep_rand(q);
		BENCH_ADD(ep_copy(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep_cmp") {
		ep_rand(p);
		ep_rand(q);
		BENCH_ADD(ep_cmp(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep_rand") {
		BENCH_ADD(ep_rand(p));
	}
	BENCH_END;
}

static void arith(void) {
	ep_t p, q, r, t[FB_BITS];
	bn_t k = NULL, l = NULL, n = NULL;

	ep_null(p);
	ep_null(q);
	ep_null(r);
	for (int i = 0; i < FB_BITS; i++) {
		ep_null(t[i]);
	}

	ep_new(p);
	ep_new(q);
	ep_new(r);
	bn_new(k);
	bn_new(n);
	bn_new(l);

	n = ep_curve_get_ord();

	BENCH_BEGIN("ep_add") {
		ep_rand(p);
		ep_rand(q);
		ep_add(p, p, q);
		ep_rand(q);
		ep_rand(p);
		ep_add(q, q, p);
		BENCH_ADD(ep_add(r, p, q));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep_add_basic") {
		ep_rand(p);
		ep_rand(q);
		BENCH_ADD(ep_add_basic(r, p, q));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
	BENCH_BEGIN("ep_add_projc") {
		ep_rand(p);
		ep_rand(q);
		ep_add_projc(p, p, q);
		ep_rand(q);
		ep_rand(p);
		ep_add_projc(q, q, p);
		BENCH_ADD(ep_add_projc(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep_add_projc (z2 = 1)") {
		ep_rand(p);
		ep_rand(q);
		ep_add_projc(p, p, q);
		ep_rand(q);
		ep_norm(q, q);
		BENCH_ADD(ep_add_projc(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep_add_projc (z1,z2 = 1)") {
		ep_rand(p);
		ep_norm(p, p);
		ep_rand(q);
		ep_norm(q, q);
		BENCH_ADD(ep_add_projc(r, p, q));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep_sub") {
		ep_rand(p);
		ep_rand(q);
		ep_add(p, p, q);
		ep_rand(q);
		ep_rand(p);
		ep_add(q, q, p);
		BENCH_ADD(ep_sub(r, p, q));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep_sub_basic") {
		ep_rand(p);
		ep_rand(q);
		BENCH_ADD(ep_sub_basic(r, p, q));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
	BENCH_BEGIN("ep_sub_projc") {
		ep_rand(p);
		ep_rand(q);
		ep_add_projc(p, p, q);
		ep_rand(q);
		ep_rand(p);
		ep_add_projc(q, q, p);
		BENCH_ADD(ep_sub_projc(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep_sub_projc (z2 = 1)") {
		ep_rand(p);
		ep_rand(q);
		ep_add_projc(p, p, q);
		ep_rand(q);
		ep_norm(q, q);
		BENCH_ADD(ep_sub_projc(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep_sub_projc (z1,z2 = 1)") {
		ep_rand(p);
		ep_norm(p, p);
		ep_rand(q);
		ep_norm(q, q);
		BENCH_ADD(ep_sub_projc(r, p, q));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep_dbl") {
		ep_rand(p);
		ep_rand(q);
		ep_add(p, p, q);
		BENCH_ADD(ep_dbl(r, p));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep_dbl_basic") {
		ep_rand(p);
		BENCH_ADD(ep_dbl_basic(r, p));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
	BENCH_BEGIN("ep_dbl_projc") {
		ep_rand(p);
		ep_rand(q);
		ep_add_projc(p, p, q);
		BENCH_ADD(ep_dbl_projc(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_dbl_projc (z1 = 1)") {
		ep_rand(p);
		ep_norm(p, p);
		BENCH_ADD(ep_dbl_projc(r, p));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep_neg") {
		ep_rand(p);
		ep_rand(q);
		ep_add(p, p, q);
		BENCH_ADD(ep_neg(r, p));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep_neg_basic") {
		ep_rand(p);
		BENCH_ADD(ep_neg_basic(r, p));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
	BENCH_BEGIN("ep_neg_projc") {
		ep_rand(p);
		ep_rand(q);
		ep_add_projc(p, p, q);
		BENCH_ADD(ep_neg_projc(r, p));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep_mul") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		BENCH_ADD(ep_mul(q, p, k));
	}
	BENCH_END;

#if EP_MUL == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep_mul_basic") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		BENCH_ADD(ep_mul_basic(q, p, k));
	}
	BENCH_END;
#endif

#if EP_MUL == WTNAF || !defined(STRIP)
	BENCH_BEGIN("ep_mul_wtnaf") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		ep_rand(p);
		BENCH_ADD(ep_mul_wtnaf(q, p, k));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep_mul_gen") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		BENCH_ADD(ep_mul_gen(q, k));
	}
	BENCH_END;

	for (int i = 0; i < FB_BITS; i++) {
		ep_new(t[i]);
	}

	BENCH_BEGIN("ep_mul_pre") {
		BENCH_ADD(ep_mul_pre(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_mul_fix") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		ep_mul_pre(t, p);
		ep_rand(p);
		BENCH_ADD(ep_mul_fix(q, t, k));
	}
	BENCH_END;

#if EP_FIX == BASIC || !defined(STRIP)
		BENCH_BEGIN("ep_mul_pre_basic") {
		BENCH_ADD(ep_mul_pre_basic(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_mul_fix_basic") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		ep_rand(p);
		ep_mul_pre_basic(t, p);
		BENCH_ADD(ep_mul_fix_basic(q, t, k));
	}
	BENCH_END;
#endif

#if EP_FIX == YAOWI || !defined(STRIP)
	BENCH_BEGIN("ep_mul_pre_yaowi") {
		BENCH_ADD(ep_mul_pre_yaowi(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_mul_fix_yaowi") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		ep_mul_pre_yaowi(t, p);
		BENCH_ADD(ep_mul_fix_yaowi(q, t, k));
	}
	BENCH_END;
#endif

#if EP_FIX == NAFWI || !defined(STRIP)
	BENCH_BEGIN("ep_mul_pre_nafwi") {
		BENCH_ADD(ep_mul_pre_nafwi(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_mul_fix_nafwi") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		ep_mul_pre_nafwi(t, p);
		BENCH_ADD(ep_mul_fix_nafwi(q, t, k));
	}
	BENCH_END;
#endif

#if EP_FIX == COMBS || !defined(STRIP)
	BENCH_BEGIN("ep_mul_pre_combs") {
		BENCH_ADD(ep_mul_pre_combs(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_mul_fix_combs") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		ep_rand(p);
		ep_mul_pre_combs(t, p);
		BENCH_ADD(ep_mul_fix_combs(q, t, k));
	}
	BENCH_END;
#endif

#if EP_FIX == COMBD || !defined(STRIP)
	BENCH_BEGIN("ep_mul_pre_combd") {
		BENCH_ADD(ep_mul_pre_combd(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_mul_fix_combd") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		ep_mul_pre_combd(t, p);
		BENCH_ADD(ep_mul_fix_combd(q, t, k));
	}
	BENCH_END;
#endif

#if EP_FIX == WTNAF || !defined(STRIP)
	BENCH_BEGIN("ep_mul_pre_wtnaf") {
		BENCH_ADD(ep_mul_pre_wtnaf(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep_mul_fix_wtnaf") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		ep_mul_pre_wtnaf(t, p);
		BENCH_ADD(ep_mul_fix_wtnaf(q, t, k));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep_mul_sim") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		ep_mul(q, p, k);
		BENCH_ADD(ep_mul_sim(r, p, k, q, l));
	}
	BENCH_END;

#if EP_SIM == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep_mul_sim_basic") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		ep_mul(q, p, k);
		BENCH_ADD(ep_mul_sim_basic(r, p, k, q, l));
	}
	BENCH_END;
#endif

#if EP_SIM == TRICK || !defined(STRIP)
	BENCH_BEGIN("ep_mul_sim_trick") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		ep_mul(q, p, k);
		BENCH_ADD(ep_mul_sim_trick(r, p, k, q, l));
	}
	BENCH_END;
#endif

#if EP_SIM == INTER || !defined(STRIP)
	BENCH_BEGIN("ep_mul_sim_inter") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		ep_mul(q, p, k);
		BENCH_ADD(ep_mul_sim_inter(r, p, k, q, l));
	}
	BENCH_END;
#endif

#if EP_SIM == JOINT || !defined(STRIP)
	BENCH_BEGIN("ep_mul_sim_joint") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		ep_mul(q, p, k);
		BENCH_ADD(ep_mul_sim_joint(r, p, k, q, l));
	}
	BENCH_END;
#endif

#if EP_SIM == INTER || !defined(STRIP)
	BENCH_BEGIN("ep_mul_sim_gen") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		ep_mul(q, p, k);
		BENCH_ADD(ep_mul_sim_gen(r, k, q, l));
	}
	BENCH_END;
#endif

	ep_free(p);
	ep_free(q);
	bn_free(k);
	bn_free(l);
	bn_free(n);
	for (int i = 0; i < FB_BITS; i++) {
		ep_free(t[i]);
	}
}

static void bench(void) {
	ep_param_print();
	util_print_banner("Utilities:", 1);
	memory();
	util();
	util_print_banner("Arithmetic:", 1);
	arith();
}

int main(void) {
	int r0, r1, r2;

	core_init();
	conf_print();
	util_print_banner("Benchmarks for the EP module:", 0);

#if defined(EP_STAND) && defined(EP_ORDIN)
	r0 = ep_param_set_any_ordin();
	if (r0 == STS_OK) {
		bench();
	}
#endif

	if (r0 == STS_ERR) {
		ep_param_set_any();
	}
	core_clean();
	return 0;
}
