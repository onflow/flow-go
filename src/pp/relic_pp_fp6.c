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
 * Implementation of the sextic extension binary field arithmetic module.
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
	fp2_t v0, v1, v2, t0, t1, t2;
	fp2_new(v0);
	fp2_new(v1);
	fp2_new(v2);
	fp2_new(t0);
	fp2_new(t1);
	fp2_new(t2);
	
	/* v0 = a0b0 */
	fp2_mul(v0, a[0], b[0]);
	
	/* v1 = a1b1 */
	fp2_mul(v1, a[1], b[1]);
	
	/* v2 = a2b2 */
	fp2_mul(v2, a[2], b[2]);
	
	/* t2 (c0) = v0 + B((a1 + a2)(b1 + b2) - v1 - v2) */
	fp2_add(t0, a[1], a[2]);
	fp2_add(t1, b[1], b[2]);
	fp2_mul(t2, t0, t1);
	fp2_sub(t2, t2, v1);
	fp2_sub(t2, t2, v2);
	fp2_mul_poly(t2, t2);
	fp2_add(t2, t2, v0);
	
	/* c1 = (a0 + a1)(b0 + b1) - v0 - v1 + Bv2 */
	fp2_add(t0, a[0], a[1]);
	fp2_add(t1, b[0], b[1]);
	fp2_mul(c[1], t0, t1);
	fp2_sub(c[1], c[1], v0);
	fp2_sub(c[1], c[1], v1);
	fp2_mul_poly(t0, v2);
	fp2_add(c[1], c[1], t0);
	
	/* c2 = (a0 + a2)(b0 + b2) - v0 + v1 - v2 */
	fp2_add(t0, a[0], a[2]);
	fp2_add(t1, b[0], b[2]);
	fp2_mul(c[2], t0, t1);
	fp2_sub(c[2], c[2], v0);
	fp2_add(c[2], c[2], v1);
	fp2_sub(c[2], c[2], v2);
	
	/* c0 = t2 */
	fp2_copy(c[0], t2);
	
	fp2_free(t2);
	fp2_free(t1);
	fp2_free(t0);
	fp2_free(v2);
	fp2_free(v1);
	fp2_free(v0);
}

void fp6_mul_fp2(fp6_t c, fp6_t a, fp2_t b) {
	fp2_mul(c[0], a[0], b);
	fp2_mul(c[1], a[1], b);
	fp2_mul(c[2], a[2], b);
}

void fp6_mul_sparse(fp6_t c, fp6_t a, fp6_t b) {
	fp2_t v0, v1, t0, t1, t2;
	fp2_new(v0);
	fp2_new(v1);
	fp2_new(t0);
	fp2_new(t1);
	fp2_new(t2);
	
	/* v0 = a0b0 */
	fp2_mul(v0, a[0], b[0]);
	
	/* v1 = a1b1 */
	fp2_mul(v1, a[1], b[1]);
	
	/* v2 = a2b2 = 0 */
	
	/* t2 (c0) = v0 + B((a1 + a2)(b1 + b2) - v1 - v2) */
	fp2_add(t0, a[1], a[2]);
	fp2_mul(t2, t0, b[1]);
	fp2_sub(t2, t2, v1);
	fp2_mul_poly(t2, t2);
	fp2_add(t2, t2, v0);
	
	/* c1 = (a0 + a1)(b0 + b1) - v0 - v1 + Bv2 */
	fp2_add(t0, a[0], a[1]);
	fp2_add(t1, b[0], b[1]);
	fp2_mul(c[1], t0, t1);
	fp2_sub(c[1], c[1], v0);
	fp2_sub(c[1], c[1], v1);
	
	/* c2 = (a0 + a2)(b0 + b2) - v0 + v1 - v2 */
	fp2_add(t0, a[0], a[2]);
	fp2_mul(c[2], t0, b[0]);
	fp2_sub(c[2], c[2], v0);
	fp2_add(c[2], c[2], v1);
	
	/* c0 = t2 */
	fp2_copy(c[0], t2);
	
	fp2_free(v0);
	fp2_free(v1);
	fp2_free(t0);
	fp2_free(t1);
	fp2_free(t2);
}

void fp6_mul_poly(fp6_t c, fp6_t a) {
	fp2_t t0;

	fp2_new(t0);
	fp2_copy(t0, a[0]);
	fp2_mul_poly(c[0], a[2]);
	fp2_copy(c[2], a[1]);
	fp2_copy(c[1], t0);

	fp2_free(t0);
}

void fp6_sqr(fp6_t c, fp6_t a) {
	fp2_t s0, s1, s2, s3, s4;
	fp2_new(s0);
	fp2_new(s1);
	fp2_new(s2);
	fp2_new(s3);
	fp2_new(s4);
	
	/* s0 = a0^2 */
	fp2_sqr(s0, a[0]);
	
	/* s1 = 2a0a1 */
	fp2_mul(s1, a[0], a[1]);
	fp2_dbl(s1, s1);
	
	/* s2 = (a0 - a1 + a2)^2 */
	fp2_sub(s2, a[0], a[1]);
	fp2_add(s2, s2, a[2]);
	fp2_sqr(s2, s2);
	
	/* s3 = 2a1a2 */
	fp2_mul(s3, a[1], a[2]);
	fp2_dbl(s3, s3);
	
	/* s4 = a2^2 */
	fp2_sqr(s4, a[2]);
	
	/* c0 = s0 + Bs3 */
	fp2_mul_poly(c[0], s3);
	fp2_add(c[0], c[0], s0);
	
	/* c1 = s1 + Bs4 */
	fp2_mul_poly(c[1], s4);
	fp2_add(c[1], c[1], s1);
	
	/* c2 = s1 + s2 + s3 - s0 - s4 */
	fp2_add(c[2], s1, s2);
	fp2_add(c[2], c[2], s3);
	fp2_sub(c[2], c[2], s0);
	fp2_sub(c[2], c[2], s4);
	
	fp2_free(s0);
	fp2_free(s1);
	fp2_free(s2);
	fp2_free(s3);
	fp2_free(s4);
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
	fp2_t v0;
	fp2_t v1;
	fp2_t v2;
	fp2_t t0;
	fp2_new(v0);
	fp2_new(v1);
	fp2_new(v2);
	fp2_new(t0);
	
	fp2_sqr(t0, a[0]);
	fp2_mul(v0, a[1], a[2]);
	fp2_mul_poly(v0, v0);
	fp2_sub(v0, t0, v0);
	
	fp2_sqr(t0, a[2]);
	fp2_mul_poly(t0, t0);
	fp2_mul(v1, a[0], a[1]);
	fp2_sub(v1, t0, v1);
	
	fp2_sqr(t0, a[1]);
	fp2_mul(v2, a[0], a[2]);
	fp2_sub(v2, t0, v2);
	
	fp2_mul(c[1], a[1], v2);
	fp2_mul_poly(c[1], c[1]);
	
	fp2_mul(c[0], a[0], v0);
	
	fp2_mul(c[2], a[2], v1);
	fp2_mul_poly(c[2], c[2]);
	
	fp2_add(t0, c[0], c[1]);
	fp2_add(t0, t0, c[2]);
	fp2_inv(t0, t0);
	
	fp2_mul(c[0], v0, t0);
	fp2_mul(c[1], v1, t0);
	fp2_mul(c[2], v2, t0);
	
	fp2_free(v0);
	fp2_free(v1);
	fp2_free(v2);
	fp2_free(t0);
}
