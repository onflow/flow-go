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
 * Implementation of the quadratic extension binary field arithmetic module.
 *
 * @version $Id$
 * @ingroup fp6
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fp6.h"
#include "relic_fp.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp6_mul(fp6_t c, fp6_t a, fp6_t b) {
	fp2_t t0, t1, t2, t3, t4, t5, t6;

	fp2_new(t0);
	fp2_new(t1);
	fp2_new(t2);
	fp2_new(t3);
	fp2_new(t4);
	fp2_new(t5);
	fp2_new(t6);

	fp2_mul(t0, a[0], b[0]);
	fp2_mul(t1, a[1], b[1]);
	fp2_mul(t2, a[2], b[2]);

	fp2_add(t3, a[0], a[1]);
	fp2_add(t4, b[0], b[1]);

	fp2_mul(t5, t3, t4);
	fp2_sub(t5, t5, t0);
	fp2_sub(t5, t5, t1);

	fp2_add(t3, a[1], a[2]);
	fp2_add(t4, b[1], b[2]);

	fp2_mul(t6, t3, t4);
	fp2_sub(t6, t6, t1);
	fp2_sub(t6, t6, t2);

	fp2_add(t3, a[0], a[2]);
	fp2_add(t4, b[0], b[2]);

	fp2_mul(t3, t3, t4);
	fp2_add(t1, t1, t3);
	fp2_sub(t1, t1, t0);
	fp2_sub(t1, t1, t2);

	fp2_mul_poly(t2, t2);
	fp2_mul_poly(t6, t6);

	fp2_sub(c[0], t0, t6);
	fp2_sub(c[1], t5, t2);
	fp2_copy(c[2], t1);

	fp_free(t0);
	fp_free(t1);
	fp_free(t2);
}

void fp6_mul_sparse1(fp6_t c, fp6_t a, fp6_t b) {
	fp2_t t0, t1, t2, t3;

	fp2_new(t0);
	fp2_new(t1);
	fp2_new(t2);
	fp2_new(t3);

	fp2_mul(t0, a[0], b[0]);
	//fp2_mul(t1, a[1], b[1]);
	//fp2_mul(t2, a[2], b[2]);

	fp2_add(t3, a[0], a[1]);
	//fp2_add(t4, b[0], b[1]);

	fp2_mul(t2, t3, b[0]);
	fp2_sub(t2, t2, t0);
	//fp2_sub(t5, t5, t1);

	fp2_add(t3, a[1], a[2]);
	//fp2_add(t4, b[1], b[2]);

	//fp2_mul(t6, t3, t4);
	//fp2_sub(t6, t6, t1);
	//fp2_sub(t6, t6, t2);

	fp2_add(t3, a[0], a[2]);
	//fp2_add(t4, b[0], b[2]);

	fp2_mul(t3, t3, b[0]);
	//fp2_add(t1, t1, t3);
	fp2_sub(t1, t3, t0);
	//fp2_sub(t1, t1, t2);

	//fp2_mul_poly(t2, t2);
	//fp2_mul_poly(t6, t6);

	//fp2_sub(c[0], t0, t6);
	//fp2_sub(c[1], t5, t2);
	fp2_copy(c[0], t0);
	fp2_copy(c[1], t2);
	fp2_copy(c[2], t1);

	fp_free(t0);
	fp_free(t1);
	fp_free(t2);
}

void fp6_mul_sparse2(fp6_t c, fp6_t a, fp6_t b) {
	fp2_t t0, t1, t2, t3, t4, t5, t6;

	fp2_new(t0);
	fp2_new(t1);
	fp2_new(t2);
	fp2_new(t3);
	fp2_new(t4);
	fp2_new(t5);
	fp2_new(t6);

	fp2_mul(t0, a[0], b[0]);
	fp2_mul(t1, a[1], b[1]);
	//fp2_mul(t2, a[2], b[2]);

	fp2_add(t3, a[0], a[1]);
	fp2_add(t4, b[0], b[1]);

	fp2_mul(t5, t3, t4);
	fp2_sub(t5, t5, t0);
	fp2_sub(t5, t5, t1);

	fp2_add(t3, a[1], a[2]);
	//fp2_add(t4, b[1], b[2]);

	fp2_mul(t6, t3, b[1]);
	fp2_sub(t6, t6, t1);
	//fp2_sub(t6, t6, t2);

	fp2_add(t3, a[0], a[2]);
	//fp2_add(t4, b[0], b[2]);

	fp2_mul(t3, t3, b[0]);
	fp2_add(t1, t1, t3);
	fp2_sub(t1, t1, t0);
	//fp2_sub(t1, t1, t2);

	fp2_mul_poly(t2, t2);
	fp2_mul_poly(t6, t6);

	fp2_sub(c[0], t0, t6);
	//fp2_sub(c[1], t5, t2);
	fp2_copy(c[1], t5);
	fp2_copy(c[2], t1);

	fp_free(t0);
	fp_free(t1);
	fp_free(t2);
}

void fp6_mul_poly(fp6_t c, fp6_t a) {
	fp2_t t0;

	fp2_new(t0);
	fp2_copy(t0, a[0]);
	fp2_mul_poly(c[2], a[2]);
	fp2_neg(c[0], c[2]);
	fp2_copy(c[2], a[1]);
	fp2_copy(c[1], t0);

	fp2_free(t0);
}

void fp6_sqr(fp6_t c, fp6_t a) {
	fp2_t t0, t1, t2, t3, t4;

	fp2_new(t0);
	fp2_new(t1);
	fp2_new(t2);
	fp2_new(t3);
	fp2_new(t4);

	fp2_sqr(t0, a[0]);
	fp2_mul(t1, a[1], a[2]);
	fp2_add(t1, t1, t1);
	fp2_sqr(t2, a[2]);
	fp2_add(t4, a[2], a[0]);
	fp2_add(t4, t4, a[1]);
	fp2_sqr(t4, t4);
	fp2_mul(t3, a[0], a[1]);
	fp2_add(t3, t3, t3);

	fp2_sub(c[2], t4, t0);
	fp2_sub(c[2], c[2], t1);
	fp2_sub(c[2], c[2], t2);
	fp2_sub(c[2], c[2], t3);

	fp2_mul_poly(t1, t1);
	fp2_mul_poly(t2, t2);
	fp2_sub(c[0], t0, t1);
	fp2_sub(c[1], t3, t2);

	fp_free(t0);
	fp_free(t1);
	fp_free(t2);
	fp_free(t3);
	fp_free(t4);
}

void fp6_frob(fp6_t c, fp6_t a, fp6_t b) {
	fp6_t t0, t1;

	fp6_new(t0);
	fp6_new(t1);

	fp6_zero(t0);
	fp6_zero(t1);

	fp2_conj(c[0], a[0]);
	fp2_conj(c[1], a[1]);
	fp2_conj(c[2], a[2]);

	fp2_copy(t1[0], c[0]);
	fp2_copy(t0[0], c[1]);
	fp6_mul(t0, t0, b);
	fp6_add(t1, t1, t0);
	fp6_zero(t0);
	fp2_copy(t0[0], c[2]);
	fp6_mul(t0, t0, b);
	fp6_mul(t0, t0, b);
	fp6_add(c, t1, t0);

	fp6_free(t0);
	fp6_free(t1);
}

void fp6_inv(fp6_t c, fp6_t a) {
	fp6_t t;
	fp2_t t0, t1;

	fp6_new(t);
	fp2_new(t0);
	fp2_new(t1);

	fp2_sqr(t0, a[0]);
	fp2_mul(t1, a[1], a[2]);
	fp2_mul_poly(t1, t1);
	fp2_add(t[0], t0, t1);

	fp2_sqr(t0, a[2]);
	fp2_mul(t1, a[0], a[1]);
	fp2_mul_poly(t0, t0);
	fp2_neg(t0, t0);
	fp2_sub(t[1], t0, t1);

	fp2_sqr(t0, a[1]);
	fp2_mul(t1, a[0], a[2]);
	fp2_sub(t[2], t0, t1);

	//f0=-txx(w.b*y.c)+w.a*y.a-txx(w.c*y.b);
	fp2_mul(t0, a[1], t[2]);
	fp2_mul_poly(t0, t0);
	fp2_neg(t0, t0);
	fp2_mul(t1, a[0], t[0]);
	fp2_add(t0, t0, t1);
	fp2_mul(t1, a[2], t[1]);
	fp2_mul_poly(t1, t1);
	fp2_sub(t0, t0, t1);
	fp2_inv(t0, t0);

	fp2_mul(t[0], t[0], t0);
	fp2_mul(t[1], t[1], t0);
	fp2_mul(t[2], t[2], t0);

	fp6_copy(c, t);

	fp2_free(t);
	fp2_free(t0);
	fp2_free(t1);
}
