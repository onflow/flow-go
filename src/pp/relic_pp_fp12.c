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
 * @ingroup fp12
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fp12.h"
#include "relic_fp.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp12_mul(fp12_t c, fp12_t a, fp12_t b) {
	fp6_t t0, t1, t2;

	fp6_new(t0);
	fp6_new(t1);
	fp6_new(t2);

	fp6_mul(t0, a[0], b[0]);
	fp6_mul(t1, a[1], b[1]);
	fp6_add(t2, b[0], b[1]);
	fp6_add(c[1], a[1], a[0]);
	fp6_mul(c[1], c[1], t2);
	fp6_sub(c[1], c[1], t0);
	fp6_sub(c[1], c[1], t1);
	fp6_mul_poly(t1, t1);
	fp6_add(c[0], t0, t1);

	fp6_free(t0);
	fp6_free(t1);
	fp6_free(t2);
}

void fp12_mul_sparse(fp12_t c, fp12_t a, fp12_t b) {
	fp6_t t0, t1, t2;

	fp6_new(t0);
	fp6_new(t1);
	fp6_new(t2);

	fp6_mul_sparse1(t0, a[0], b[0]);
	fp6_mul_sparse2(t1, a[1], b[1]);
	fp6_add(t2, b[0], b[1]);
	fp6_add(c[1], a[1], a[0]);
	fp6_mul_sparse2(c[1], c[1], t2);
	fp6_sub(c[1], c[1], t0);
	fp6_sub(c[1], c[1], t1);
	fp6_mul_poly(t1, t1);
	fp6_add(c[0], t0, t1);

	fp6_free(t0);
	fp6_free(t1);
	fp6_free(t2);
}

void fp12_sqr(fp12_t c, fp12_t a) {
	fp6_t t0, t1;

	fp6_new(t0);
	fp6_new(t1);

    fp6_add(t0, a[0], a[1]);
    fp6_mul_poly(t1, a[1]);
    fp6_add(t1, a[0], t1);
    fp6_mul(t0, t0, t1);
    fp6_mul(c[1], a[0], a[1]);
    fp6_sub(c[0], t0, c[1]);
    fp6_mul_poly(t1, c[1]);
    fp6_sub(c[0], c[0], t1);
    fp6_add(c[1], c[1], c[1]);

	fp6_free(t0);
	fp6_free(t1);
}

/*ZZn6 t=b; t*=t;
b+=a; b*=b;
b-=t;
a=tx(t);
b-=a;
a+=a; a+=one();
b-=one();*/

void fp12_sqr_uni(fp12_t c, fp12_t a) {
	fp6_t t0, t1;
	fp_t one;

	fp6_new(t0);
	fp6_new(t1);
	fp_new(one);

	fp6_sqr(t0, a[1]); //t = b * b
	fp6_add(t1, a[0], a[1]); //b = b + a
	fp6_sqr(t1, t1); //t1 = b * b
	fp6_sub(t1, t1, t0); //b = b - t
	fp6_mul_poly(c[0], t0); //a = tx(t)
	fp6_sub(t1, t1, c[0]); //b = b - a
	fp6_add(c[0], c[0], c[0]); //a = a + a
	fp_set_dig(one, 1);
	fp_add(c[0][0][0], c[0][0][0], one);
	fp6_copy(c[1], t1);
	fp_sub(c[1][0][0], c[1][0][0], one); // b = b -  1

	fp6_free(t0);
	fp6_free(t1);
	fp_free(one);
}

void fp12_frob(fp12_t c, fp12_t a, fp12_t b) {
	fp12_t t, t0, t1;

	fp12_new(t);
	fp12_new(t0);
	fp12_new(t1);

	fp12_sqr(t, b);
	fp6_frob(t0[0], a[0], t[0]);
	fp6_frob(t1[0], a[1], t[0]);
	fp6_zero(t0[1]);
	fp6_zero(t1[1]);
	fp12_mul(t1, t1, b);
	fp12_add(c, t0, t1);
	fp12_free(t);
	fp12_free(t0);
	fp12_free(t1);
}

void fp12_conj(fp12_t c, fp12_t a) {
	fp6_copy(c[0], a[0]);
	fp6_neg(c[1], a[1]);
}

void fp12_inv(fp12_t c, fp12_t a) {
	fp6_t t0, t1, t2;

	fp6_new(t0);
	fp6_new(t1);
	fp6_new(t2);
	fp6_sqr(t0, a[0]);
	fp6_sqr(t1, a[1]);
	fp6_mul_poly(t2, t1);
	fp6_sub(t0, t0, t2);
	fp6_inv(t0, t0);
	fp12_conj(c, a);
	fp6_mul(c[1], c[1], t0);
	fp6_mul(c[0], c[0], t0);

	fp6_free(t0);
	fp6_free(t1);
	fp6_free(t2);
}

void fp12_exp(fp12_t c, fp12_t a, bn_t b) {
	fp12_t tab[16], t, u;
	dig_t buf;
	int bitbuf, bitcpy, bitcnt, mode, digidx, i, j, w;

	w = 4;

	for (i = 0; i < 16; i++) {
		fp12_new(tab[i]);
	}

	fp12_new(t);
	fp12_new(u);
	fp12_set_dig(t, 1);
	fp12_copy(tab[1], a);

	/* Compute the value at tab[1<<(w-1)] by squaring tab[1] (w-1) times. */
	fp12_copy(tab[1 << (w - 1)], tab[1]);
	for (i = 0; i < (w - 1); i++) {
		fp12_sqr(tab[1 << (w - 1)], tab[1 << (w - 1)]);
	}

	/* Create upper table. */
	for (i = (1 << (w - 1)) + 1; i < (1 << w); i++) {
		fp12_mul(tab[i], tab[i - 1], tab[1]);
	}

	/* Set initial mode and bit count. */
	mode = 0;
	bitcnt = 1;
	buf = 0;
	digidx = b->used - 1;
	bitcpy = 0;
	bitbuf = 0;

	for (;;) {
		/* Grab next digit as required. */
		if (--bitcnt == 0) {
			/* If digidx == -1 we are out of digits so break. */
			if (digidx == -1) {
				break;
			}
			/* Read next digit and set bitcnt. */
			buf = b->dp[digidx--];
			bitcnt = (int)BN_DIGIT;
		}

		/* Grab the next most significant bit from the exponent. */
		j = (buf >> (FP_DIGIT - 1)) & 0x01;
		buf <<= (dig_t)1;

		if (mode == 0 && j == 0) {
			continue;
		}

		/* If the bit is zero and mode == 1 then we square. */
		if (mode == 1 && j == 0) {
			fp12_sqr(t, t);
			continue;
		}

		/* Else we add it to the window. */
		bitbuf |= (j << (w - ++bitcpy));
		mode = 2;

		if (bitcpy == w) {
			/* Window is filled so square as required and multiply. */
			for (i = 0; i < w; i++) {
				fp12_sqr(t, t);
			}
			fp12_mul(t, t, tab[bitbuf]);
			bitcpy = 0;
			bitbuf = 0;
			mode = 1;
		}
	}

	/* If bits remain then square/multiply. */
	if (mode == 2 && bitcpy > 0) {
		/* Square then multiply if the bit is set. */
		for (i = 0; i < bitcpy; i++) {
			fp12_sqr(t, t);

			/* Get next bit of the window. */
			bitbuf <<= 1;
			if ((bitbuf & (1 << w)) != 0) {
				fp12_mul(t, t, tab[1]);
			}
		}
	}

	fp12_copy(c, t);
	for (i = 0; i < 16; i++) {
		fp12_free(tab[i]);
	}
	fp12_free(u);
	fp12_free(t);
}

void fp12_exp_basic(fp12_t c, fp12_t a, bn_t b) {
	fp12_t t;

	fp12_new(t);

	fp12_copy(t, a);

	for (int i = bn_bits(b) - 2; i >= 0; i--) {
		fp12_sqr(t, t);
		if (bn_test_bit(b, i)) {
			fp12_mul(t, t, a);
		}
	}
	fp12_copy(c, t);

	fp12_free(t);
}

void fp12_exp_basic_uni(fp12_t c, fp12_t a, bn_t b) {
	fp12_t t;

	fp12_new(t);

	fp12_copy(t, a);

	for (int i = bn_bits(b) - 2; i >= 0; i--) {
		fp12_sqr_uni(t, t);
		if (bn_test_bit(b, i)) {
			fp12_mul(t, t, a);
		}
	}
	fp12_copy(c, t);

	fp12_free(t);
}

