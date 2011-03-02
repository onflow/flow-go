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
	fb2_t a[BENCH];

	BENCH_SMALL("fb2_null", fb2_null(a[i]));

	BENCH_SMALL("fb2_new", fb2_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		fb2_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		fb2_new(a[i]);
	}
	BENCH_SMALL("fb2_free", fb2_free(a[i]));

	(void)a;
}

static void util2(void) {
	fb2_t a, b;

	fb2_null(a);
	fb2_null(b);

	fb2_new(a);
	fb2_new(b);

	BENCH_BEGIN("fb2_copy") {
		fb2_rand(a);
		BENCH_ADD(fb2_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_neg") {
		fb2_rand(a);
		BENCH_ADD(fb2_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_zero") {
		fb2_rand(a);
		BENCH_ADD(fb2_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_is_zero") {
		fb2_rand(a);
		BENCH_ADD((void)fb2_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_rand") {
		BENCH_ADD(fb2_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_cmp") {
		fb2_rand(a);
		fb2_rand(b);
		BENCH_ADD(fb2_cmp(b, a));
	}
	BENCH_END;

	fb2_free(a);
	fb2_free(b);
}

static void arith2(void) {
	fb2_t a, b, c;

	fb2_new(a);
	fb2_new(b);
	fb2_new(c);

	BENCH_BEGIN("fb2_add") {
		fb2_rand(a);
		fb2_rand(b);
		BENCH_ADD(fb2_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_sub") {
		fb2_rand(a);
		fb2_rand(b);
		BENCH_ADD(fb2_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_mul") {
		fb2_rand(a);
		fb2_rand(b);
		BENCH_ADD(fb2_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_sqr") {
		fb2_rand(a);
		BENCH_ADD(fb2_sqr(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb2_inv") {
		fb2_rand(a);
		BENCH_ADD(fb2_inv(c, a));
	}
	BENCH_END;

	fb2_free(a);
	fb2_free(b);
	fb2_free(c);
}

static void memory4(void) {
	fb4_t a[BENCH];

	BENCH_SMALL("fb4_null", fb4_null(a[i]));

	BENCH_SMALL("fb4_new", fb4_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		fb4_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		fb4_new(a[i]);
	}
	BENCH_SMALL("fb4_free", fb4_free(a[i]));

	(void)a;
}

static void util4(void) {
	fb4_t a, b;

	fb4_null(a);
	fb4_null(b);

	fb4_new(a);
	fb4_new(b);

	BENCH_BEGIN("fb4_copy") {
		fb4_rand(a);
		BENCH_ADD(fb4_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_neg") {
		fb4_rand(a);
		BENCH_ADD(fb4_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_zero") {
		fb4_rand(a);
		BENCH_ADD(fb4_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_is_zero") {
		fb4_rand(a);
		BENCH_ADD((void)fb4_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_rand") {
		BENCH_ADD(fb4_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_cmp") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_cmp(b, a));
	}
	BENCH_END;

	fb4_free(a);
	fb4_free(b);
}

static void arith4(void) {
	fb4_t a, b, c;
	bn_t d;

	fb4_new(a);
	fb4_new(b);
	fb4_new(c);
	bn_new(d);

	BENCH_BEGIN("fb4_add") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_sub") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_mul") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_mul_dxd") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_mul_dxd(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_mul_dxs") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_mul_dxs(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_mul_sxs") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_mul_sxs(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_sqr") {
		fb4_rand(a);
		BENCH_ADD(fb4_sqr(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_inv") {
		fb4_rand(a);
		BENCH_ADD(fb4_inv(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_frb") {
		fb4_rand(a);
		BENCH_ADD(fb4_frb(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_exp") {
		fb4_rand(a);
		bn_rand(d, BN_POS, FB_BITS);
		BENCH_ADD(fb4_exp(c, a, d));
	}
	BENCH_END;

	fb4_free(a);
	fb4_free(b);
	fb4_free(c);
	bn_free(d);
}

static void memory6(void) {
	fb6_t a[BENCH];

	BENCH_SMALL("fb6_null", fb6_null(a[i]));

	BENCH_SMALL("fb6_new", fb6_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		fb6_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		fb6_new(a[i]);
	}
	BENCH_SMALL("fb6_free", fb6_free(a[i]));

	(void)a;
}

static void util6(void) {
	fb6_t a, b;

	fb6_null(a);
	fb6_null(b);

	fb6_new(a);
	fb6_new(b);

	BENCH_BEGIN("fb6_copy") {
		fb6_rand(a);
		BENCH_ADD(fb6_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_neg") {
		fb6_rand(a);
		BENCH_ADD(fb6_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_zero") {
		fb6_rand(a);
		BENCH_ADD(fb6_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_is_zero") {
		fb6_rand(a);
		BENCH_ADD((void)fb6_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_rand") {
		BENCH_ADD(fb6_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_cmp") {
		fb6_rand(a);
		fb6_rand(b);
		BENCH_ADD(fb6_cmp(b, a));
	}
	BENCH_END;

	fb6_free(a);
	fb6_free(b);
}

static void arith6(void) {
	fb6_t a, b, c;

	fb6_new(a);
	fb6_new(b);
	fb6_new(c);

	BENCH_BEGIN("fb6_add") {
		fb6_rand(a);
		fb6_rand(b);
		BENCH_ADD(fb6_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_sub") {
		fb6_rand(a);
		fb6_rand(b);
		BENCH_ADD(fb6_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_mul") {
		fb6_rand(a);
		fb6_rand(b);
		BENCH_ADD(fb6_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_sqr") {
		fb6_rand(a);
		BENCH_ADD(fb6_sqr(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb6_inv") {
		fb6_rand(a);
		BENCH_ADD(fb6_inv(c, a));
	}
	BENCH_END;

	fb6_free(a);
	fb6_free(b);
	fb6_free(c);
}

static void memory12(void) {
	fb12_t a[BENCH];

	BENCH_SMALL("fb12_null", fb12_null(a[i]));

	BENCH_SMALL("fb12_new", fb12_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		fb12_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		fb12_new(a[i]);
	}
	BENCH_SMALL("fb12_free", fb12_free(a[i]));

	(void)a;
}

static void util12(void) {
	fb12_t a, b;

	fb12_null(a);
	fb12_null(b);

	fb12_new(a);
	fb12_new(b);

	BENCH_BEGIN("fb12_copy") {
		fb12_rand(a);
		BENCH_ADD(fb12_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_neg") {
		fb12_rand(a);
		BENCH_ADD(fb12_neg(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_zero") {
		fb12_rand(a);
		BENCH_ADD(fb12_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_is_zero") {
		fb12_rand(a);
		BENCH_ADD((void)fb12_is_zero(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_rand") {
		BENCH_ADD(fb12_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_cmp") {
		fb12_rand(a);
		fb12_rand(b);
		BENCH_ADD(fb12_cmp(b, a));
	}
	BENCH_END;

	fb12_free(a);
	fb12_free(b);
}

static void arith12(void) {
	fb12_t a, b, c;

	fb12_new(a);
	fb12_new(b);
	fb12_new(c);

	BENCH_BEGIN("fb12_add") {
		fb12_rand(a);
		fb12_rand(b);
		BENCH_ADD(fb12_add(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_sub") {
		fb12_rand(a);
		fb12_rand(b);
		BENCH_ADD(fb12_sub(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_mul") {
		fb12_rand(a);
		fb12_rand(b);
		BENCH_ADD(fb12_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_sqr") {
		fb12_rand(a);
		BENCH_ADD(fb12_sqr(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb12_inv") {
		fb12_rand(a);
		BENCH_ADD(fb12_inv(c, a));
	}
	BENCH_END;

	fb12_free(a);
	fb12_free(b);
	fb12_free(c);
}

#ifdef WITH_EB

static void pairing(void) {
	eb_t p, q;
	fb4_t r;

	fb4_new(r);
	eb_new(p);
	eb_new(q);

#if PB_MAP == ETATS || !defined(STRIP)
	BENCH_BEGIN("pb_map_etats") {
		eb_rand(p);
		eb_rand(q);
		BENCH_ADD(pb_map_etats(r, p, q));
	}
	BENCH_END;
#endif

#if PB_MAP == ETATN || !defined(STRIP)
	BENCH_BEGIN("pb_map_etatn") {
		eb_rand(p);
		eb_rand(q);
		BENCH_ADD(pb_map_etatn(r, p, q));
	}
	BENCH_END;
#endif

	eb_free(p);
	eb_free(q);
	fb4_free(r);
}

#endif

#ifdef WITH_HB

static void pairing2(void) {
	hb_t p, q;
	fb12_t r;

	fb12_new(r);
	hb_new(p);
	hb_new(q);

	BENCH_BEGIN("pb_map_etat2 (d,d)") {
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(pb_map_etat2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_etat2 (d,n)") {
		hb_rand_deg(p);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_etat2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_etat2 (n,d)") {
		hb_rand_deg(p);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_etat2(r, q, p));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_etat2 (n,n)") {
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_etat2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_oeta2 (d,d)") {
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(pb_map_oeta2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_oeta2 (n,d)") {
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(pb_map_oeta2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_oeta2 (n,d)") {
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(pb_map_oeta2(r, q, p));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_oeta2 (n,n)") {
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_oeta2(r, p, q));
	}
	BENCH_END;

	hb_free(p);
	hb_free(q);
	fb12_free(r);
}

#endif

int main(void) {
	int r0, r1;
	core_init();
	conf_print();

	util_print_banner("Benchmarks for the PB module:", 0);

	fb_param_set_any();
	fb_param_print();

	util_print_banner("Quadratic extension:", 0);
	util_print_banner("Utilities:", 1);
	memory2();
	util2();

	util_print_banner("Arithmetic:", 1);
	arith2();

	util_print_banner("Quartic extension:", 0);
	util_print_banner("Utilities", 1)
			memory4();
	util4();

	util_print_banner("Arithmetic:", 1);
	arith4();

	util_print_banner("Sextic extension:", 0);
	util_print_banner("Utilities", 1)
			memory6();
	util6();

	util_print_banner("Arithmetic:", 1);
	arith6();

	util_print_banner("Dodecic extension:", 0);
	util_print_banner("Utilities", 1)
			memory12();
	util12();

	util_print_banner("Arithmetic:", 1);
	arith12();

	r0 = r1 = STS_ERR;
#ifdef WITH_EB
	r0 = eb_param_set_any_super();
	if (r0 == STS_OK) {
		eb_param_print();
		util_print_banner("Arithmetic:", 1);
		pairing();
	}
#endif

#ifdef WITH_HB
	r1 = hb_param_set_any_super();
	if ( r1 == STS_OK) {
		hb_param_print();
		util_print_banner("Arithmetic:", 1);
		pairing2();
	}
#endif

	if (r0 == STS_ERR && r1 == STS_ERR) {
		THROW(ERR_NO_CURVE);
	}

	core_clean();
	return 0;
}
