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

static void memory2(void) {
	fp2_t a[BENCH];

	BENCH_SMALL("fp2_null", fp2_null(a[i]));

	BENCH_SMALL("fp2_new", fp2_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		fp2_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		fp2_new(a[i]);
	}
	BENCH_SMALL("fp2_free", fp2_free(a[i]));

	(void)a;
}

static void util2(void) {
	fp2_t a, b;

	fp2_null(a);
	fp2_null(b);

	fp2_new(a);
	fp2_new(b);

	BENCH_BEGIN("fp2_copy") {
		fp2_rand(a);
		BENCH_ADD(fp2_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_neg") {
		fp2_rand(a);
		BENCH_ADD(fp2_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_zero") {
		fp2_rand(a);
		BENCH_ADD(fp2_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_is_zero") {
		fp2_rand(a);
		BENCH_ADD((void)fp2_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_rand") {
		BENCH_ADD(fp2_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_cmp") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_cmp(b, a));
	}
	BENCH_END;

	fp2_free(a);
	fp2_free(b);
}

static void arith2(void) {
	fp2_t a, b, c;
	bn_t d;

	fp2_new(a);
	fp2_new(b);
	fp2_new(c);
	bn_new(d);

	BENCH_BEGIN("fp2_add") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_add(c, a, b));
	}
	BENCH_END;

#if PP_EXT == BASIC || !defined(STRIP)
	BENCH_BEGIN("fp2_add_basic") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_add_basic(c, a, b));
	}
	BENCH_END;
#endif

#if PP_EXT == LOWER || !defined(STRIP)
	BENCH_BEGIN("fp2_add_lower") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_add_lower(c, a, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fp2_sub") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_sub(c, a, b));
	}
	BENCH_END;

#if PP_EXT == BASIC || !defined(STRIP)
	BENCH_BEGIN("fp2_sub_basic") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_sub_basic(c, a, b));
	}
	BENCH_END;
#endif

#if PP_EXT == LOWER || !defined(STRIP)
	BENCH_BEGIN("fp2_sub_lower") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_sub_lower(c, a, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fp2_dbl") {
		fp2_rand(a);
		BENCH_ADD(fp2_dbl(c, a));
	}
	BENCH_END;

#if PP_EXT == BASIC || !defined(STRIP)
	BENCH_BEGIN("fp2_dbl_basic") {
		fp2_rand(a);
		BENCH_ADD(fp2_dbl_basic(c, a));
	}
	BENCH_END;
#endif

#if PP_EXT == LOWER || !defined(STRIP)
	BENCH_BEGIN("fp2_dbl_lower") {
		fp2_rand(a);
		BENCH_ADD(fp2_dbl_lower(c, a));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fp2_mul") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_mul(c, a, b));
	}
	BENCH_END;

#if PP_EXT == BASIC || !defined(STRIP)
	BENCH_BEGIN("fp2_mul_basic") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_mul_basic(c, a, b));
	}
	BENCH_END;
#endif

#if PP_EXT == LOWER || !defined(STRIP)
	BENCH_BEGIN("fp2_mul_lower") {
		fp2_rand(a);
		fp2_rand(b);
		BENCH_ADD(fp2_mul_lower(c, a, b));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fp2_mul_art") {
		fp2_rand(a);
		BENCH_ADD(fp2_mul_art(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_mul_nor") {
		fp2_rand(a);
		BENCH_ADD(fp2_mul_nor(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_sqr") {
		fp2_rand(a);
		BENCH_ADD(fp2_sqr(c, a));
	}
	BENCH_END;

#if PP_EXT == BASIC || !defined(STRIP)
	BENCH_BEGIN("fp2_sqr_basic") {
		fp2_rand(a);
		BENCH_ADD(fp2_sqr_basic(c, a));
	}
	BENCH_END;
#endif

#if PP_EXT == LOWER || !defined(STRIP)
	BENCH_BEGIN("fp2_sqr_lower") {
		fp2_rand(a);
		BENCH_ADD(fp2_sqr_lower(c, a));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("fp2_inv") {
		fp2_rand(a);
		BENCH_ADD(fp2_inv(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_exp") {
		fp2_rand(a);
		d->used = FP_DIGS;
		dv_copy(d->dp, fp_prime_get(), FP_DIGS);
		BENCH_ADD(fp2_exp(c, a, d));
	}
	BENCH_END;

	BENCH_BEGIN("fp2_frb") {
		fp2_rand(a);
		BENCH_ADD(fp2_frb(c, a));
	}
	BENCH_END;

	fp2_free(a);
	fp2_free(b);
	fp2_free(c);
}

static void memory6(void) {
	fp6_t a[BENCH];

	BENCH_SMALL("fp6_null", fp6_null(a[i]));

	BENCH_SMALL("fp6_new", fp6_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		fp6_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		fp6_new(a[i]);
	}
	BENCH_SMALL("fp6_free", fp6_free(a[i]));

	(void)a;
}

static void util6(void) {
	fp6_t a, b;

	fp6_null(a);
	fp6_null(b);

	fp6_new(a);
	fp6_new(b);

	BENCH_BEGIN("fp6_copy") {
		fp6_rand(a);
		BENCH_ADD(fp6_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_neg") {
		fp6_rand(a);
		BENCH_ADD(fp6_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_zero") {
		fp6_rand(a);
		BENCH_ADD(fp6_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_is_zero") {
		fp6_rand(a);
		BENCH_ADD((void)fp6_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_rand") {
		BENCH_ADD(fp6_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_cmp") {
		fp6_rand(a);
		fp6_rand(b);
		BENCH_ADD(fp6_cmp(b, a));
	}
	BENCH_END;

	fp6_free(a);
	fp6_free(b);
}

static void arith6(void) {
	fp6_t a, b, c;
	bn_t d;

	fp6_new(a);
	fp6_new(b);
	fp6_new(c);
	bn_new(d);

	BENCH_BEGIN("fp6_add") {
		fp6_rand(a);
		fp6_rand(b);
		BENCH_ADD(fp6_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_sub") {
		fp6_rand(a);
		fp6_rand(b);
		BENCH_ADD(fp6_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_dbl") {
		fp6_rand(a);
		BENCH_ADD(fp6_dbl(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_mul") {
		fp6_rand(a);
		fp6_rand(b);
		BENCH_ADD(fp6_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_mul_dxs") {
		fp6_rand(a);
		fp6_rand(b);
		BENCH_ADD(fp6_mul_dxs(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_mul_dxq") {
		fp6_rand(a);
		fp6_rand(b);
		BENCH_ADD(fp6_mul_dxq(c, a, b[0]));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_mul_art") {
		fp6_rand(a);
		BENCH_ADD(fp6_mul_art(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_sqr") {
		fp6_rand(a);
		BENCH_ADD(fp6_sqr(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_inv") {
		fp6_rand(a);
		BENCH_ADD(fp6_inv(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_exp") {
		fp6_rand(a);
		d->used = FP_DIGS;
		dv_copy(d->dp, fp_prime_get(), FP_DIGS);
		BENCH_ADD(fp6_exp(c, a, d));
	}
	BENCH_END;

	BENCH_BEGIN("fp6_frb") {
		fp6_rand(a);
		BENCH_ADD(fp6_frb(c, a));
	}
	BENCH_END;

	fp6_free(a);
	fp6_free(b);
	fp6_free(c);
}

static void memory12(void) {
	fp12_t a[BENCH];

	BENCH_SMALL("fp12_null", fp12_null(a[i]));

	BENCH_SMALL("fp12_new", fp12_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		fp12_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		fp12_new(a[i]);
	}
	BENCH_SMALL("fp12_free", fp12_free(a[i]));

	(void)a;
}

static void util12(void) {
	fp12_t a, b;

	fp12_null(a);
	fp12_null(b);

	fp12_new(a);
	fp12_new(b);

	BENCH_BEGIN("fp12_copy") {
		fp12_rand(a);
		BENCH_ADD(fp12_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_neg") {
		fp12_rand(a);
		BENCH_ADD(fp12_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_zero") {
		fp12_rand(a);
		BENCH_ADD(fp12_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_is_zero") {
		fp12_rand(a);
		BENCH_ADD((void)fp12_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_rand") {
		BENCH_ADD(fp12_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_cmp") {
		fp12_rand(a);
		fp12_rand(b);
		BENCH_ADD(fp12_cmp(b, a));
	}
	BENCH_END;

	fp12_free(a);
	fp12_free(b);
}

static void arith12(void) {
	fp12_t a, b, c;
	bn_t d;

	fp12_new(a);
	fp12_new(b);
	fp12_new(c);
	bn_new(d);

	BENCH_BEGIN("fp12_add") {
		fp12_rand(a);
		fp12_rand(b);
		BENCH_ADD(fp12_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_sub") {
		fp12_rand(a);
		fp12_rand(b);
		BENCH_ADD(fp12_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_mul") {
		fp12_rand(a);
		fp12_rand(b);
		BENCH_ADD(fp12_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_mul_dxs") {
		fp12_rand(a);
		fp12_rand(b);
		BENCH_ADD(fp12_mul_dxs(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_sqr") {
		fp12_rand(a);
		BENCH_ADD(fp12_sqr(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_sqr_uni") {
		fp12_rand(a);
		BENCH_ADD(fp12_sqr_uni(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_inv") {
		fp12_rand(a);
		BENCH_ADD(fp12_inv(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_inv_uni") {
		fp12_rand(a);
		BENCH_ADD(fp12_inv_uni(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_exp") {
		fp12_rand(a);
		d->used = FP_DIGS;
		dv_copy(d->dp, fp_prime_get(), FP_DIGS);
		BENCH_ADD(fp12_exp(c, a, d));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_exp_uni") {
		fp12_rand(a);
		d->used = FP_DIGS;
		dv_copy(d->dp, fp_prime_get(), FP_DIGS);
		BENCH_ADD(fp12_exp_uni(c, a, d));
	}
	BENCH_END;

	BENCH_BEGIN("fp12_frb") {
		fp12_rand(a);
		BENCH_ADD(fp12_frb(c, a));
	}
	BENCH_END;

	fp12_free(a);
	fp12_free(b);
	fp12_free(c);
}

static void memory(void) {
	ep2_t a[BENCH];

	BENCH_SMALL("ep2_null", ep2_null(a[i]));

	BENCH_SMALL("ep2_new", ep2_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		ep2_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		ep2_new(a[i]);
	}
	BENCH_SMALL("ep2_free", ep2_free(a[i]));

	(void)a;
}

static void util(void) {
	ep2_t p, q;

	ep2_null(p);
	ep2_null(q);

	ep2_new(p);
	ep2_new(q);

	BENCH_BEGIN("ep2_is_infty") {
		ep2_rand(p);
		BENCH_ADD(ep2_is_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_set_infty") {
		ep2_rand(p);
		BENCH_ADD(ep2_set_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_copy") {
		ep2_rand(p);
		ep2_rand(q);
		BENCH_ADD(ep2_copy(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_cmp") {
		ep2_rand(p);
		ep2_rand(q);
		BENCH_ADD(ep2_cmp(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_rand") {
		BENCH_ADD(ep2_rand(p));
	}
	BENCH_END;
}

static void arith(void) {
	ep2_t p, q, r;
	ep_t _q;
	bn_t k, n;
	fp12_t e;
	fp2_t s, t;

	ep2_null(p);
	ep2_null(q);
	ep2_null(r);
	ep_null(_q);
	bn_null(k);
	bn_null(n);
	fp12_null(e);
	fp2_null(s);
	fp2_null(t);

	ep2_new(p);
	ep2_new(q);
	ep2_new(r);
	ep_new(_q);
	bn_new(k);
	bn_new(n);
	fp12_new(e);
	fp2_new(s);
	fp2_new(t);

	ep2_curve_get_ord(n);

	BENCH_BEGIN("ep2_add") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add(p, p, q);
		ep2_rand(q);
		ep2_rand(p);
		ep2_add(q, q, p);
		BENCH_ADD(ep2_add(r, p, q));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep2_add_basic") {
		ep2_rand(p);
		ep2_rand(q);
		BENCH_ADD(ep2_add_basic(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_add_slp_basic") {
		ep2_rand(p);
		ep2_rand(q);
		BENCH_ADD(ep2_add_slp_basic(r, s, p, q));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
	BENCH_BEGIN("ep2_add_projc") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add_projc(p, p, q);
		ep2_rand(q);
		ep2_rand(p);
		ep2_add_projc(q, q, p);
		BENCH_ADD(ep2_add_projc(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_add_slp_projc") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add_projc(p, p, q);
		ep2_rand(q);
		ep2_rand(p);
		ep2_add_projc(q, q, p);
		BENCH_ADD(ep2_add_slp_projc(r, s, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_add_projc (z2 = 1)") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add_projc(p, p, q);
		ep2_rand(q);
		ep2_norm(q, q);
		BENCH_ADD(ep2_add_projc(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_add_projc (z1,z2 = 1)") {
		ep2_rand(p);
		ep2_norm(p, p);
		ep2_rand(q);
		ep2_norm(q, q);
		BENCH_ADD(ep2_add_projc(r, p, q));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep2_sub") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add(p, p, q);
		ep2_rand(q);
		ep2_rand(p);
		ep2_add(q, q, p);
		BENCH_ADD(ep2_sub(r, p, q));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep2_sub_basic") {
		ep2_rand(p);
		ep2_rand(q);
		BENCH_ADD(ep2_sub_basic(r, p, q));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
	BENCH_BEGIN("ep2_sub_projc") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add_projc(p, p, q);
		ep2_rand(q);
		ep2_rand(p);
		ep2_add_projc(q, q, p);
		BENCH_ADD(ep2_sub_projc(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_sub_projc (z2 = 1)") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add_projc(p, p, q);
		ep2_rand(q);
		ep2_norm(q, q);
		BENCH_ADD(ep2_sub_projc(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_sub_projc (z1,z2 = 1)") {
		ep2_rand(p);
		ep2_norm(p, p);
		ep2_rand(q);
		ep2_norm(q, q);
		BENCH_ADD(ep2_sub_projc(r, p, q));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep2_dbl") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add(p, p, q);
		BENCH_ADD(ep2_dbl(r, p));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep2_dbl_basic") {
		ep2_rand(p);
		BENCH_ADD(ep2_dbl_basic(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_dbl_slp_basic") {
		ep2_rand(p);
		BENCH_ADD(ep2_dbl_slp_basic(r, s, t, p));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
	BENCH_BEGIN("ep2_dbl_projc") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add_projc(p, p, q);
		BENCH_ADD(ep2_dbl_projc(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_dbl_slp_projc") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add_projc(p, p, q);
		BENCH_ADD(ep2_dbl_slp_projc(r, s, t, p));
	}
	BENCH_END;

	BENCH_BEGIN("ep2_dbl_projc (z1 = 1)") {
		ep2_rand(p);
		ep2_norm(p, p);
		BENCH_ADD(ep2_dbl_projc(r, p));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep2_neg") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add(p, p, q);
		BENCH_ADD(ep2_neg(r, p));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("ep2_neg_basic") {
		ep2_rand(p);
		BENCH_ADD(ep2_neg_basic(r, p));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
	BENCH_BEGIN("ep2_neg_projc") {
		ep2_rand(p);
		ep2_rand(q);
		ep2_add_projc(p, p, q);
		BENCH_ADD(ep2_neg_projc(r, p));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("ep2_mul") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		BENCH_ADD(ep2_mul(q, p, k));
	}
	BENCH_END;

	BENCH_BEGIN("pp_map") {
		ep2_rand(p);
		ep_rand(_q);
		BENCH_ADD(pp_map(e, p, _q));
	}
	BENCH_END;

#if PP_MAP == R_ATE || !defined(STRIP)
	BENCH_BEGIN("pp_map_r_ate") {
		ep2_rand(p);
		ep_rand(_q);
		BENCH_ADD(pp_map_r_ate(e, p, _q));
	}
	BENCH_END;
#endif

#if PP_MAP == O_ATE || !defined(STRIP)
	BENCH_BEGIN("pp_map_o_ate") {
		ep2_rand(p);
		ep_rand(_q);
		BENCH_ADD(pp_map_o_ate(e, p, _q));
	}
	BENCH_END;
#endif

#if PP_MAP == X_ATE || !defined(STRIP)
	BENCH_BEGIN("pp_map_x_ate") {
		ep2_rand(p);
		ep_rand(_q);
		BENCH_ADD(pp_map_x_ate(e, p, _q));
	}
	BENCH_END;
#endif

	ep2_free(p);
	ep2_free(q);
	ep2_free(r);
	ep_free(_q);
	bn_free(k);
	bn_free(n);
	fp12_free(e);
	fp2_free(s);
	fp2_free(t);
}

int main(void) {
	core_init();
	conf_print();

	util_print_banner("Benchmarks for the PP module:", 0);

	fp_param_set_any_tower();
	fp_param_print();

	util_print_banner("Quadratic extension:", 0);
	util_print_banner("Utilities:", 1);
	memory2();
	util2();

	util_print_banner("Arithmetic:", 1);
	arith2();

	util_print_banner("Sextic extension:", 0);
	util_print_banner("Utilities:", 1);
	memory6();
	util6();

	util_print_banner("Arithmetic:", 1);
	arith6();

	util_print_banner("Dodecic extension:", 0);
	util_print_banner("Utilities:", 1);
	memory12();
	util12();

	util_print_banner("Arithmetic:", 1);
	arith12();

	if (ep_param_set_any_pairf() == STS_OK) {
		ep_param_print();
		util_print_banner("Arithmetic:", 1);
		memory();
		util();
		arith();
	} else {
		THROW(ERR_NO_CURVE);
	}

	core_clean();
	return 0;
}
