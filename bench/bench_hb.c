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
 * Benchmarks for the binary hyperelliptic curve module.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

static void memory(void) {
	hb_t a[BENCH];

	BENCH_SMALL("hb_null", hb_null(a[i]));

	BENCH_SMALL("hb_new", hb_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		hb_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		hb_new(a[i]);
	}
	BENCH_SMALL("hb_free", hb_free(a[i]));

	(void)a;
}

static void util(void) {
	hb_t p, q;

	hb_null(p);
	hb_null(q);

	hb_new(p);
	hb_new(q);

	BENCH_BEGIN("hb_is_infty") {
		hb_rand(p);
		BENCH_ADD(hb_is_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_set_infty") {
		hb_rand(p);
		BENCH_ADD(hb_set_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_copy") {
		hb_rand(p);
		hb_rand(q);
		BENCH_ADD(hb_copy(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_cmp") {
		hb_rand(p);
		hb_rand(q);
		BENCH_ADD(hb_cmp(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_rand") {
		BENCH_ADD(hb_rand(p));
	}
	BENCH_END;
}

static void arith(void) {
	hb_t p, q, r, tab[HB_TABLE_MAX];
	bn_t k, l, n;

	hb_null(p);
	hb_null(q);
	hb_null(r);
	for (int i = 0; i < HB_TABLE_MAX; i++) {
		hb_null(tab[i]);
	}
	bn_null(k);
	bn_null(l);
	bn_null(n);

	hb_new(p);
	hb_new(q);
	hb_new(r);
	bn_new(k);
	bn_new(n);
	bn_new(l);

	hb_curve_get_ord(n);

	BENCH_BEGIN("hb_add (n,n)") {
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_add(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_add (n,d)") {
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_add(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_add (d,d)") {
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_add(r, p, q));
	}
	BENCH_END;

#if HB_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("hb_add_basic (n,n)") {
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_add_basic(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_add_basic (n,d)") {
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_add_basic(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_add_basic (d,d)") {
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(hb_add_basic(r, p, q));
	}
	BENCH_END;
#endif

	//#if HB_ADD == PROJC || !defined(STRIP)
	//  BENCH_BEGIN("hb_add_projc") {
	//      hb_rand(p);
	//      hb_rand(q);
	//      hb_add_projc(p, p, q);
	//      hb_rand(q);
	//      hb_rand(p);
	//      hb_add_projc(q, q, p);
	//      BENCH_ADD(hb_add_projc(r, p, q));
	//  }
	//  BENCH_END;
	//
	//  BENCH_BEGIN("hb_add_projc (z2 = 1)") {
	//      hb_rand(p);
	//      hb_rand(q);
	//      hb_add_projc(p, p, q);
	//      hb_rand(q);
	//      hb_norm(q, q);
	//      BENCH_ADD(hb_add_projc(r, p, q));
	//  }
	//  BENCH_END;
	//
	//  BENCH_BEGIN("hb_add_projc (z1,z2 = 1)") {
	//      hb_rand(p);
	//      hb_norm(p, p);
	//      hb_rand(q);
	//      hb_norm(q, q);
	//      BENCH_ADD(hb_add_projc(r, p, q));
	//  }
	//  BENCH_END;
	//#endif

	BENCH_BEGIN("hb_sub (n,n)") {
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		hb_add(p, p, q);
		hb_rand(q);
		hb_rand(p);
		hb_add(q, q, p);
		BENCH_ADD(hb_sub(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_sub (n,d)") {
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		hb_add(p, p, q);
		hb_rand(q);
		hb_rand(p);
		hb_add(q, q, p);
		BENCH_ADD(hb_sub(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_sub (d,d)") {
		hb_rand_deg(p);
		hb_rand_deg(q);
		hb_add(p, p, q);
		hb_rand(q);
		hb_rand(p);
		hb_add(q, q, p);
		BENCH_ADD(hb_sub(r, p, q));
	}
	BENCH_END;

#if HB_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("hb_sub_basic (n,n)") {
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_sub_basic(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_sub_basic (n,d)") {
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_sub_basic(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("hb_sub_basic (d,d)") {
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(hb_sub_basic(r, p, q));
	}
	BENCH_END;
#endif

	//#if HB_ADD == PROJC || !defined(STRIP)
	//  BENCH_BEGIN("hb_sub_projc") {
	//      hb_rand(p);
	//      hb_rand(q);
	//      hb_add_projc(p, p, q);
	//      hb_rand(q);
	//      hb_rand(p);
	//      hb_add_projc(q, q, p);
	//      BENCH_ADD(hb_sub_projc(r, p, q));
	//  }
	//  BENCH_END;
	//
	//  BENCH_BEGIN("hb_sub_projc (z2 = 1)") {
	//      hb_rand(p);
	//      hb_rand(q);
	//      hb_add_projc(p, p, q);
	//      hb_rand(q);
	//      hb_norm(q, q);
	//      BENCH_ADD(hb_sub_projc(r, p, q));
	//  }
	//  BENCH_END;
	//
	//  BENCH_BEGIN("hb_sub_projc (z1,z2 = 1)") {
	//      hb_rand(p);
	//      hb_norm(p, p);
	//      hb_rand(q);
	//      hb_norm(q, q);
	//      BENCH_ADD(hb_sub_projc(r, p, q));
	//  }
	//  BENCH_END;
	//#endif

	BENCH_BEGIN("hb_dbl (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_dbl(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_dbl (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_dbl(r, p));
	}
	BENCH_END;

#if HB_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("hb_dbl_basic (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_dbl_basic(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_dbl_basic (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_dbl_basic(r, p));
	}
	BENCH_END;
#endif

	//#if HB_ADD == PROJC || !defined(STRIP)
	//  BENCH_BEGIN("hb_dbl_projc") {
	//      hb_rand(p);
	//      hb_rand(q);
	//      hb_add_projc(p, p, q);
	//      BENCH_ADD(hb_dbl_projc(r, p));
	//  }
	//  BENCH_END;
	//
	//  BENCH_BEGIN("hb_dbl_projc (z1 = 1)") {
	//      hb_rand(p);
	//      hb_norm(p, p);
	//      BENCH_ADD(hb_dbl_projc(r, p));
	//  }
	//  BENCH_END;
	//#endif

	BENCH_BEGIN("hb_neg (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_neg(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_neg (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_neg(r, p));
	}
	BENCH_END;

#if HB_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("hb_neg_basic") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_neg_basic(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_neg_basic") {
		hb_rand_deg(p);
		BENCH_ADD(hb_neg_basic(r, p));
	}
	BENCH_END;
#endif

	//#if HB_ADD == PROJC || !defined(STRIP)
	//  BENCH_BEGIN("hb_neg_projc") {
	//      hb_rand(p);
	//      hb_rand(q);
	//      hb_add_projc(p, p, q);
	//      BENCH_ADD(hb_neg_projc(r, p));
	//  }
	//  BENCH_END;
	//#endif

	BENCH_BEGIN("hb_oct (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_oct(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_oct (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_oct(r, p));
	}
	BENCH_END;

#if HB_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("hb_oct_basic (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_oct_basic(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_oct_basic (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_oct_basic(r, p));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("hb_mul (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_rand(q);
		BENCH_ADD(hb_mul(q, p, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_rand(q);
		BENCH_ADD(hb_mul(q, p, k));
	}
	BENCH_END;

#if HB_MUL == BASIC || !defined(STRIP)
	BENCH_BEGIN("hb_mul_basic (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		//BENCH_ADD(hb_mul_basic(q, p, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_basic (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		//BENCH_ADD(hb_mul_basic(q, p, k));
	}
	BENCH_END;
#endif

#if HB_MUL == OCTUP || !defined(STRIP)
	if (hb_curve_is_super()) {
		BENCH_BEGIN("hb_mul_octup (n)") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_rand_non(p, 0);
			BENCH_ADD(hb_mul_octup(q, p, k));
		}
		BENCH_END;
	}

	if (hb_curve_is_super()) {
		BENCH_BEGIN("hb_mul_octup (d)") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_rand_deg(p);
			BENCH_ADD(hb_mul_octup(q, p, k));
		}
		BENCH_END;
	}
#endif

#if HB_MUL == LWNAF || !defined(STRIP)
	BENCH_BEGIN("hb_mul_lwnaf (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_lwnaf(q, p, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_lwnaf (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_lwnaf(q, p, k));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("hb_mul_gen") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		BENCH_ADD(hb_mul_gen(q, k));
	}
	BENCH_END;

	for (int i = 0; i < HB_TABLE; i++) {
		hb_new(tab[i]);
	}

	BENCH_BEGIN("hb_mul_pre (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_pre(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_mul_pre(tab, p);
		BENCH_ADD(hb_mul_fix(q, tab, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_pre (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_pre(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_mul_pre(tab, p);
		BENCH_ADD(hb_mul_fix(q, tab, k));
	}
	BENCH_END;

	for (int i = 0; i < HB_TABLE; i++) {
		hb_free(tab[i]);
	}

#if HB_FIX == BASIC || !defined(STRIP)
	for (int i = 0; i < HB_TABLE_BASIC; i++) {
		hb_new(tab[i]);
	}
	BENCH_BEGIN("hb_mul_pre_basic (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_pre_basic(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_basic (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_mul_pre_basic(tab, p);
		BENCH_ADD(hb_mul_fix_basic(q, tab, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_pre_basic (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_pre_basic(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_basic (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_mul_pre_basic(tab, p);
		BENCH_ADD(hb_mul_fix_basic(q, tab, k));
	}
	BENCH_END;
	for (int i = 0; i < HB_TABLE_BASIC; i++) {
		hb_free(tab[i]);
	}
#endif

#if HB_FIX == YAOWI || !defined(STRIP)
	for (int i = 0; i < HB_TABLE_YAOWI; i++) {
		hb_new(tab[i]);
	}
	BENCH_BEGIN("hb_mul_pre_yaowi (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_pre_yaowi(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_yaowi (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_mul_pre_yaowi(tab, p);
		BENCH_ADD(hb_mul_fix_yaowi(q, tab, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_pre_yaowi (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_pre_yaowi(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_yaowi (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_mul_pre_yaowi(tab, p);
		BENCH_ADD(hb_mul_fix_yaowi(q, tab, k));
	}
	BENCH_END;
	for (int i = 0; i < HB_TABLE_YAOWI; i++) {
		hb_free(tab[i]);
	}
#endif

#if HB_FIX == NAFWI || !defined(STRIP)
	for (int i = 0; i < HB_TABLE_NAFWI; i++) {
		hb_new(tab[i]);
	}
	BENCH_BEGIN("hb_mul_pre_nafwi (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_pre_nafwi(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_nafwi (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_mul_pre_nafwi(tab, p);
		BENCH_ADD(hb_mul_fix_nafwi(q, tab, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_pre_nafwi (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_pre_nafwi(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_nafwi (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_mul_pre_nafwi(tab, p);
		BENCH_ADD(hb_mul_fix_nafwi(q, tab, k));
	}
	BENCH_END;
	for (int i = 0; i < HB_TABLE_NAFWI; i++) {
		hb_free(tab[i]);
	}
#endif

#if HB_FIX == COMBS || !defined(STRIP)
	for (int i = 0; i < HB_TABLE_COMBS; i++) {
		hb_new(tab[i]);
	}
	BENCH_BEGIN("hb_mul_pre_combs (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_pre_combs(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_combs (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_mul_pre_combs(tab, p);
		BENCH_ADD(hb_mul_fix_combs(q, tab, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_pre_combs (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_pre_combs(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_combs (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_mul_pre_combs(tab, p);
		BENCH_ADD(hb_mul_fix_combs(q, tab, k));
	}
	BENCH_END;
	for (int i = 0; i < HB_TABLE_COMBS; i++) {
		hb_free(tab[i]);
	}
#endif

#if HB_FIX == COMBD || !defined(STRIP)
	for (int i = 0; i < HB_TABLE_COMBD; i++) {
		hb_new(tab[i]);
	}
	BENCH_BEGIN("hb_mul_pre_combd (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_pre_combd(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_combd (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_mul_pre_combd(tab, p);
		BENCH_ADD(hb_mul_fix_combd(q, tab, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_pre_combd (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_pre_combd(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_combd (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_mul_pre_combd(tab, p);
		BENCH_ADD(hb_mul_fix_combd(q, tab, k));
	}
	BENCH_END;
	for (int i = 0; i < HB_TABLE_COMBD; i++) {
		hb_free(tab[i]);
	}
#endif

#if HB_FIX == LWNAF || !defined(STRIP)
	for (int i = 0; i < HB_TABLE_LWNAF; i++) {
		hb_new(tab[i]);
	}
	BENCH_BEGIN("hb_mul_pre_lwnaf (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_pre_lwnaf(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_lwnaf (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_mul_pre_lwnaf(tab, p);
		BENCH_ADD(hb_mul_fix_lwnaf(q, tab, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_pre_lwnaf (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_pre_lwnaf(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_lwnaf (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_mul_pre_lwnaf(tab, p);
		BENCH_ADD(hb_mul_fix_lwnaf(q, tab, k));
	}
	BENCH_END;
	for (int i = 0; i < HB_TABLE_LWNAF; i++) {
		hb_free(tab[i]);
	}
#endif

#if HB_FIX == OCTUP || !defined(STRIP)
	for (int i = 0; i < HB_TABLE_OCTUP; i++) {
		hb_new(tab[i]);
	}
	BENCH_BEGIN("hb_mul_pre_octup (n)") {
		hb_rand_non(p, 0);
		BENCH_ADD(hb_mul_pre_octup(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_octup (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_non(p, 0);
		hb_mul_pre_octup(tab, p);
		BENCH_ADD(hb_mul_fix_octup(q, tab, k));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_pre_octup (d)") {
		hb_rand_deg(p);
		BENCH_ADD(hb_mul_pre_octup(tab, p));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_fix_octup (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		hb_rand_deg(p);
		hb_mul_pre_octup(tab, p);
		BENCH_ADD(hb_mul_fix_octup(q, tab, k));
	}
	BENCH_END;
	for (int i = 0; i < HB_TABLE_OCTUP; i++) {
		hb_free(tab[i]);
	}
#endif

	BENCH_BEGIN("hb_mul_sim (n,n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_mul_sim(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim (n,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim (d,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim(r, p, k, q, l));
	}
	BENCH_END;

#if HB_SIM == BASIC || !defined(STRIP)
	BENCH_BEGIN("hb_mul_sim_basic (n,n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_mul_sim_basic(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_basic (n,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_basic(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_basic (d,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_basic(r, p, k, q, l));
	}
	BENCH_END;
#endif

#if HB_SIM == TRICK || !defined(STRIP)
	BENCH_BEGIN("hb_mul_sim_trick (n,n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_mul_sim_trick(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_trick (n,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_trick(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_trick (d,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_trick(r, p, k, q, l));
	}
	BENCH_END;
#endif

#if HB_SIM == INTER || !defined(STRIP)
	BENCH_BEGIN("hb_mul_sim_inter (n,n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_mul_sim_inter(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_inter (n,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_inter(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_inter (d,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_inter(r, p, k, q, l));
	}
	BENCH_END;

#endif

#if HB_SIM == JOINT || !defined(STRIP)
	BENCH_BEGIN("hb_mul_sim_joint (n,n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_mul_sim_joint(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_joint (n,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_joint(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_joint (d,d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_joint(r, p, k, q, l));
	}
	BENCH_END;
#endif

	BENCH_BEGIN("hb_mul_sim_gen (n)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_non(q, 0);
		BENCH_ADD(hb_mul_sim_gen(r, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("hb_mul_sim_gen (d)") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		hb_rand_deg(q);
		BENCH_ADD(hb_mul_sim_gen(r, k, q, l));
	}
	BENCH_END;

	hb_free(p);
	hb_free(q);
	bn_free(k);
	bn_free(l);
	bn_free(n);
}

static void bench(void) {
	hb_param_print();
	util_banner("Utilities:", 1);
	memory();
	util();
	util_banner("Arithmetic:", 1);
	arith();
}

int main(void) {
	int r0;

	core_init();
	conf_print();
	util_banner("Benchmarks for the HB module:", 0);

#if defined(HB_SUPER)
	r0 = hb_param_set_any_super();
	if (r0 == STS_OK) {
		bench();
	}
#endif

	if (r0 == STS_ERR) {
		if (hb_param_set_any() == STS_ERR) {
			THROW(ERR_NO_CURVE);
			core_clean();
			return 1;
		}
	}
	core_clean();
	return 0;
}
