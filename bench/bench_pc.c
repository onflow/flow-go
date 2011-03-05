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
 * Benchmarks for the binary elliptic curve module.
 *
 * @version $Id: bench_ec.c 366 2010-06-04 06:23:16Z dfaranha $
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

static void memory1(void) {
	g1_t a[BENCH];

	BENCH_SMALL("g1_null", g1_null(a[i]));

	BENCH_SMALL("g1_new", g1_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		g1_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		g1_new(a[i]);
	}
	BENCH_SMALL("g1_free", g1_free(a[i]));

	(void)a;
}

static void util1(void) {
	g1_t p, q;

	g1_null(p);
	g1_null(q);

	g1_new(p);
	g1_new(q);

	BENCH_BEGIN("g1_is_infty") {
		g1_rand(p);
		BENCH_ADD(g1_is_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("g1_set_infty") {
		g1_rand(p);
		BENCH_ADD(g1_set_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("g1_copy") {
		g1_rand(p);
		g1_rand(q);
		BENCH_ADD(g1_copy(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("g1_cmp") {
		g1_rand(p);
		g1_rand(q);
		BENCH_ADD(g1_cmp(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("g1_rand") {
		BENCH_ADD(g1_rand(p));
	}
	BENCH_END;
}

static void arith1(void) {
	g1_t p, q, r, t[G1_TABLE];
	bn_t k, l, n;

	g1_null(p);
	g1_null(q);
	g1_null(r);
	for (int i = 0; i < G1_TABLE; i++) {
		g1_null(t[i]);
	}

	g1_new(p);
	g1_new(q);
	g1_new(r);
	bn_new(k);
	bn_new(n);
	bn_new(l);

	g1_get_ord(n);

	BENCH_BEGIN("g1_add") {
		g1_rand(p);
		g1_rand(q);
		g1_add(p, p, q);
		g1_rand(q);
		g1_rand(p);
		g1_add(q, q, p);
		BENCH_ADD(g1_add(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("g1_sub") {
		g1_rand(p);
		g1_rand(q);
		g1_add(p, p, q);
		g1_rand(q);
		g1_rand(p);
		g1_add(q, q, p);
		BENCH_ADD(g1_sub(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("g1_dbl") {
		g1_rand(p);
		g1_rand(q);
		g1_add(p, p, q);
		BENCH_ADD(g1_dbl(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("g1_neg") {
		g1_rand(p);
		g1_rand(q);
		g1_add(p, p, q);
		BENCH_ADD(g1_neg(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("g1_mul") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		g1_rand(p);
		BENCH_ADD(g1_mul(q, p, k));
	}
	BENCH_END;

	BENCH_BEGIN("g1_mul_gen") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		BENCH_ADD(g1_mul_gen(q, k));
	}
	BENCH_END;

	for (int i = 0; i < G1_TABLE; i++) {
		g1_new(t[i]);
	}

	BENCH_BEGIN("g1_mul_pre") {
		BENCH_ADD(g1_mul_pre(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("g1_mul_fix") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		g1_mul_pre(t, p);
		BENCH_ADD(g1_mul_fix(q, t, k));
	}
	BENCH_END;

	BENCH_BEGIN("g1_mul_sim") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		g1_rand(p);
		g1_rand(q);
		BENCH_ADD(g1_mul_sim(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("g1_mul_sim_gen") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		g1_rand(q);
		BENCH_ADD(g1_mul_sim_gen(r, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("g1_map") {
		unsigned char msg[5];
		rand_bytes(msg, 5);
		BENCH_ADD(g1_map(p, msg, 5));
	} BENCH_END;

	g1_free(p);
	g1_free(q);
	bn_free(k);
	bn_free(l);
	bn_free(n);
	for (int i = 0; i < G1_TABLE; i++) {
		g1_free(t[i]);
	}
}

static void memory2(void) {
	g2_t a[BENCH];

	BENCH_SMALL("g2_null", g2_null(a[i]));

	BENCH_SMALL("g2_new", g2_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		g2_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		g2_new(a[i]);
	}
	BENCH_SMALL("g2_free", g2_free(a[i]));

	(void)a;
}

static void util2(void) {
	g2_t p, q;

	g2_null(p);
	g2_null(q);

	g2_new(p);
	g2_new(q);

	BENCH_BEGIN("g2_is_infty") {
		g2_rand(p);
		BENCH_ADD(g2_is_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("g2_set_infty") {
		g2_rand(p);
		BENCH_ADD(g2_set_infty(p));
	}
	BENCH_END;

	BENCH_BEGIN("g2_copy") {
		g2_rand(p);
		g2_rand(q);
		BENCH_ADD(g2_copy(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("g2_cmp") {
		g2_rand(p);
		g2_rand(q);
		BENCH_ADD(g2_cmp(p, q));
	}
	BENCH_END;

	BENCH_BEGIN("g2_rand") {
		BENCH_ADD(g2_rand(p));
	}
	BENCH_END;
}

static void arith2(void) {
	g2_t p, q, r, t[G1_TABLE];
	bn_t k, l, n;

	g2_null(p);
	g2_null(q);
	g2_null(r);
	for (int i = 0; i < G1_TABLE; i++) {
		g2_null(t[i]);
	}

	g2_new(p);
	g2_new(q);
	g2_new(r);
	bn_new(k);
	bn_new(n);
	bn_new(l);

	g2_get_ord(n);

	BENCH_BEGIN("g2_add") {
		g2_rand(p);
		g2_rand(q);
		g2_add(p, p, q);
		g2_rand(q);
		g2_rand(p);
		g2_add(q, q, p);
		BENCH_ADD(g2_add(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("g2_sub") {
		g2_rand(p);
		g2_rand(q);
		g2_add(p, p, q);
		g2_rand(q);
		g2_rand(p);
		g2_add(q, q, p);
		BENCH_ADD(g2_sub(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("g2_dbl") {
		g2_rand(p);
		g2_rand(q);
		g2_add(p, p, q);
		BENCH_ADD(g2_dbl(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("g2_neg") {
		g2_rand(p);
		g2_rand(q);
		g2_add(p, p, q);
		BENCH_ADD(g2_neg(r, p));
	}
	BENCH_END;

	BENCH_BEGIN("g2_mul") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		g2_rand(p);
		BENCH_ADD(g2_mul(q, p, k));
	}
	BENCH_END;

	BENCH_BEGIN("g2_mul_gen") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		BENCH_ADD(g2_mul_gen(q, k));
	}
	BENCH_END;

	for (int i = 0; i < G1_TABLE; i++) {
		g2_new(t[i]);
	}

	BENCH_BEGIN("g2_mul_pre") {
		BENCH_ADD(g2_mul_pre(t, p));
	}
	BENCH_END;

	BENCH_BEGIN("g2_mul_fix") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		g2_mul_pre(t, p);
		BENCH_ADD(g2_mul_fix(q, t, k));
	}
	BENCH_END;

	BENCH_BEGIN("g2_mul_sim") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		g2_rand(p);
		g2_rand(q);
		BENCH_ADD(g2_mul_sim(r, p, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("g2_mul_sim_gen") {
		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);
		bn_rand(l, BN_POS, bn_bits(n));
		bn_mod(l, l, n);
		g2_rand(q);
		BENCH_ADD(g2_mul_sim_gen(r, k, q, l));
	}
	BENCH_END;

	BENCH_BEGIN("g2_map") {
		unsigned char msg[5];
		rand_bytes(msg, 5);
		BENCH_ADD(g2_map(p, msg, 5));
	} BENCH_END;

	g2_free(p);
	g2_free(q);
	bn_free(k);
	bn_free(l);
	bn_free(n);
	for (int i = 0; i < G1_TABLE; i++) {
		g2_free(t[i]);
	}
}

static void memory(void) {
	gt_t a[BENCH];

	BENCH_SMALL("gt_null", gt_null(a[i]));

	BENCH_SMALL("gt_new", gt_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		gt_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		gt_new(a[i]);
	}
	BENCH_SMALL("gt_free", gt_free(a[i]));

	(void)a;
}

static void util(void) {
	gt_t a, b;

	gt_null(a);
	gt_null(b);

	gt_new(a);
	gt_new(b);

	BENCH_BEGIN("gt_copy") {
		gt_rand(a);
		BENCH_ADD(gt_copy(b, a));
	}
	BENCH_END;

	BENCH_BEGIN("gt_set_unity") {
		gt_rand(a);
		BENCH_ADD(gt_set_unity(a));
	}
	BENCH_END;

	BENCH_BEGIN("gt_is_unity") {
		gt_rand(a);
		BENCH_ADD((void)gt_is_unity(a));
	}
	BENCH_END;

	BENCH_BEGIN("gt_rand") {
		BENCH_ADD(gt_rand(a));
	}
	BENCH_END;

	BENCH_BEGIN("gt_cmp") {
		gt_rand(a);
		gt_rand(b);
		BENCH_ADD(gt_cmp(b, a));
	}
	BENCH_END;

	gt_free(a);
	gt_free(b);
}

static void arith(void) {
	gt_t a, b, c;
	g1_t p;
	g2_t q;
	bn_t d;

	gt_new(a);
	gt_new(b);
	gt_new(c);
	g1_new(p);
	g2_new(q);
	bn_new(d);

	BENCH_BEGIN("gt_mul") {
		gt_rand(a);
		gt_rand(b);
		BENCH_ADD(gt_mul(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("gt_sqr") {
		gt_rand(a);
		gt_rand(b);
		BENCH_ADD(gt_sqr(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("gt_inv") {
		gt_rand(a);
		BENCH_ADD(gt_inv(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("gt_exp") {
		gt_rand(a);
		g1_get_ord(d);
		BENCH_ADD(gt_exp(c, a, d));
	}
	BENCH_END;

	BENCH_BEGIN("pc_map") {
		g1_rand(p);
		g2_rand(q);
		BENCH_ADD(pc_map(a, p, q));
	}
	BENCH_END;

	gt_free(a);
	gt_free(b);
	gt_free(c);
	g1_free(p);
	g2_free(q);
}

static void bench(void) {
	pc_param_print();
	util_print_banner("Group G_1:", 0);
	util_print_banner("Utilities:", 1);
	memory1();
	util1();
	util_print_banner("Arithmetic:", 1);
	arith1();
	util_print_banner("Group G_2:", 0);
	util_print_banner("Utilities:", 1);
	memory2();
	util2();
	util_print_banner("Arithmetic:", 1);
	arith2();
	util_print_banner("Group G_T:", 0);
	util_print_banner("Utilities:", 1);
	memory();
	util();
	util_print_banner("Arithmetic:", 1);
	arith();
}

int main(void) {
	core_init();
	conf_print();
	util_print_banner("Benchmarks for the PC module:", 0);

	if (pc_param_set_any() == STS_OK) {
		bench();
	} else {
		THROW(ERR_NO_CURVE);
	}

	core_clean();
	return 0;
}
