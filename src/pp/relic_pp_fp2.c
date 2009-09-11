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
 * @ingroup fp2
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fp2.h"
#include "relic_fp.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp2_mul(fp2_t c, fp2_t a, fp2_t b) {
	fp_t t0, t1, t2;

	fp_new(t0);
	fp_new(t1);
	fp_new(t2);

	fp_mul(t0, a[0], b[0]);
	fp_mul(t1, a[1], b[1]);
	fp_add(t2, a[0], a[1]);
	fp_add(c[1], b[0], b[1]);
	fp_mul(c[1], c[1], t2);
	fp_sub(c[1], c[1], t0);
	fp_sub(c[1], c[1], t1);
	fp_sub(c[0], t0, t1);

	if (fp_prime_get_qnr() == -2) {
		fp_sub(c[0], c[0], t1);
	}

	fp_free(t0);
	fp_free(t1);
	fp_free(t2);
}

void fp2_mul_qnr(fp2_t c, fp2_t a) {
	fp_t t0;

	fp_new(t0);

	fp_copy(t0, a[0]);
	fp_neg(c[0], a[1]);

	if (fp_prime_get_qnr() == -2) {
		fp_dbl(c[0], c[0]);
	}
	fp_copy(c[1], t0);

	fp_free(t0);
}

void fp2_conj(fp2_t c, fp2_t a) {
	fp_copy(c[0], a[0]);
	fp_neg(c[1], a[1]);
}

void fp2_mul_poly(fp2_t c, fp2_t a) {
	fp2_t t0;

	fp2_new(t0);

	switch (fp_prime_get_mod8()[0]) {
		case 5:
			/* If p = 5 mod 8, x^2 - sqrt(sqrt(-2)) is irreducible. */
			fp2_mul_qnr(c, a);
			break;
		case 3:
			/* If p = 3 mod 8, x^2 - sqrt(1 + sqrt(-1)) is irreducible. */
			fp2_copy(t0, a);
			fp2_mul_qnr(t0, t0);
			fp2_add(c, t0, a);
			break;
		case 7:
			/* If p = 7 mod 8 and p = 2,3 mod 5, x^2 - sqrt(2 + sqrt(-1)) is
			 * irreducible. */
			fp2_copy(t0, a);
			fp2_mul_qnr(a, a);
			fp2_mul(c, a, t0);
			fp2_mul(c, c, t0);
			break;
	}

	fp2_free(t0);
}

void fp2_sqr(fp2_t c, fp2_t a) {
	fp_t t0, t1;

	fp_new(t0);
	fp_new(t1);

	fp_add(t0, a[0], a[1]);

	if (fp_prime_get_qnr() == -1) {
		fp_sub(t1, a[0], a[1]);
	} else {
		if (fp_prime_get_qnr() == -2) {
			fp_dbl(t1, a[1]);
			fp_sub(t1, a[0], t1);
		}
	}

	fp_mul(c[1], a[0], a[1]);
	fp_mul(c[0], t0, t1);

	if (fp_prime_get_qnr() == -2) {
		fp_add(c[0], c[0], c[1]);
	}

	fp_dbl(c[1], c[1]);

	fp_free(t0);
	fp_free(t1);
}

void fp2_inv(fp2_t c, fp2_t a) {
	fp_t t0, t1;

	fp_new(t0);
	fp_new(t1);

	fp_sqr(t0, a[0]);
	fp_sqr(t1, a[1]);
	fp_add(t0, t0, t1);

	fp_inv(t1, t0);

	fp_mul(c[0], a[0], t1);
	fp_neg(t1, t1);
	fp_mul(c[1], a[1], t1);

	fp_free(t0);
	fp_free(t1);
}
