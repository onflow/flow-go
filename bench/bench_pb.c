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
	fb2_t a, b, c;

	fb2_null(a);
	fb2_null(b);
	fb2_null(c);

	fb2_new(a);
	fb2_new(b);
	fb2_new(c);

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
	fb2_free(c);
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

	fb_free(a);
	fb_free(b);
	fb_free(c);
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
	fb4_t a, b, c;

	fb4_null(a);
	fb4_null(b);
	fb4_null(c);

	fb4_new(a);
	fb4_new(b);
	fb4_new(c);

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
	fb4_free(c);
}

static void arith4(void) {
	fb4_t a, b, c;

	fb4_new(a);
	fb4_new(b);
	fb4_new(c);

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

	BENCH_BEGIN("fb4_mul_dexde") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_mul_dexde(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_mul_dexsp") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_mul_dexsp(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_mul_spxsp") {
		fb4_rand(a);
		fb4_rand(b);
		BENCH_ADD(fb4_mul_spxsp(c, a, b));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_sqr") {
		fb4_rand(a);
		BENCH_ADD(fb4_sqr(c, a));
	}
	BENCH_END;

	BENCH_BEGIN("fb4_exp_2m") {
		fb4_rand(a);
		BENCH_ADD(fb4_exp_2m(c, a));
	}
	BENCH_END;

	fb_free(a);
	fb_free(b);
	fb_free(c);
}

static void arith(void) {
	eb_t p, q;
	fb4_t r;

	fb4_new(r);
	eb_new(p);
	eb_new(q);

	BENCH_BEGIN("pb_map") {
		eb_rand(p);
		eb_rand(q);
		BENCH_ADD(pb_map(r, p, q));
	}
	BENCH_END;

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

int main(void) {
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

	if (eb_param_set_any_super() == STS_OK) {
		eb_param_print();
		util_print_banner("Arithmetic:", 1);
		arith();
	} else {
		THROW(ERR_NO_CURVE);
	}

	core_clean();
	return 0;
}
