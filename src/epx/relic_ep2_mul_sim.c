/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2014 RELIC Authors
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
 * Implementation of simultaneous point multiplication on a prime elliptic
 * curve over a quadratic extension.
 *
 * @version $Id$
 * @ingroup epx
 */

#include "relic_core.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EP_SIM == INTER || !defined(STRIP)

static void ep2_mul_sim_plain(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l,
		ep2_t *t) {
	int len, l0, l1, i, n0, n1, w, gen;
	int8_t naf0[2 * FP_BITS + 1], naf1[2 * FP_BITS + 1], *_k, *_m;
	ep2_t t0[1 << (EP_WIDTH - 2)];
	ep2_t t1[1 << (EP_WIDTH - 2)];

	for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
		ep2_null(t0[i]);
		ep2_null(t1[i]);
	}

	TRY {
		gen = (t == NULL ? 0 : 1);
		if (!gen) {
			for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
				ep2_new(t0[i]);
			}
			ep2_tab(t0, p, EP_WIDTH);
			t = (ep2_t *)t0;
		}

		/* Prepare the precomputation table. */
		for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
			ep2_new(t1[i]);
		}
		/* Compute the precomputation table. */
		ep2_tab(t1, q, EP_WIDTH);

		/* Compute the w-TNAF representation of k. */
		if (gen) {
			w = EP_DEPTH;
		} else {
			w = EP_WIDTH;
		}

		l0 = l1 = 2 * FP_BITS + 1;
		bn_rec_naf(naf0, &l0, k, w);
		bn_rec_naf(naf1, &l1, l, EP_WIDTH);

		len = MAX(l0, l1);
		_k = naf0 + len - 1;
		_m = naf1 + len - 1;
		for (i = l0; i < len; i++)
			naf0[i] = 0;
		for (i = l1; i < len; i++)
			naf1[i] = 0;

		ep2_set_infty(r);
		for (i = len - 1; i >= 0; i--, _k--, _m--) {
			ep2_dbl(r, r);

			n0 = *_k;
			n1 = *_m;
			if (n0 > 0) {
				ep2_add(r, r, t[n0 / 2]);
			}
			if (n0 < 0) {
				ep2_sub(r, r, t[-n0 / 2]);
			}
			if (n1 > 0) {
				ep2_add(r, r, t1[n1 / 2]);
			}
			if (n1 < 0) {
				ep2_sub(r, r, t1[-n1 / 2]);
			}
		}
		/* Convert r to affine coordinates. */
		ep2_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		/* Free the precomputation tables. */
		if (!gen) {
			for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
				ep2_free(t0[i]);
			}
		}
		for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
			ep2_free(t1[i]);
		}
	}
}

#endif /* EP_SIM == INTER */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_SIM == BASIC || !defined(STRIP)

void ep2_mul_sim_basic(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l) {
	ep2_t t;

	ep2_null(t);

	TRY {
		ep2_new(t);
		ep2_mul(t, q, l);
		ep2_mul(r, p, k);
		ep2_add(t, t, r);
		ep2_norm(r, t);

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t);
	}
}

#endif

#if EP_SIM == TRICK || !defined(STRIP)

void ep2_mul_sim_trick(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l) {
	ep2_t t0[1 << (EP_WIDTH / 2)];
	ep2_t t1[1 << (EP_WIDTH / 2)];
	ep2_t t[1 << EP_WIDTH];
	bn_t n;
	int l0, l1, w = EP_WIDTH / 2;
	uint8_t w0[CEIL(2 * FP_BITS, w)], w1[CEIL(2 * FP_BITS, w)];

	bn_null(n);

	for (int i = 0; i < 1 << EP_WIDTH; i++) {
		ep2_null(t[i]);
	}

	for (int i = 0; i < 1 << (EP_WIDTH / 2); i++) {
		ep2_null(t0[i]);
		ep2_null(t1[i]);
	}

	TRY {
		bn_new(n);

		ep2_curve_get_ord(n);

		for (int i = 0; i < (1 << w); i++) {
			ep2_new(t0[i]);
			ep2_new(t1[i]);
		}
		for (int i = 0; i < (1 << EP_WIDTH); i++) {
			ep2_new(t[i]);
		}

		ep2_set_infty(t0[0]);
		for (int i = 1; i < (1 << w); i++) {
			ep2_add(t0[i], t0[i - 1], p);
		}

		ep2_set_infty(t1[0]);
		for (int i = 1; i < (1 << w); i++) {
			ep2_add(t1[i], t1[i - 1], q);
		}

		for (int i = 0; i < (1 << w); i++) {
			for (int j = 0; j < (1 << w); j++) {
				ep2_add(t[(i << w) + j], t0[i], t1[j]);
			}
		}

		l0 = l1 = CEIL(2 * FP_BITS, w);
		bn_rec_win(w0, &l0, k, w);
		bn_rec_win(w1, &l1, l, w);

		for (int i = l0; i < l1; i++) {
			w0[i] = 0;
		}
		for (int i = l1; i < l0; i++) {
			w1[i] = 0;
		}

		ep2_set_infty(r);
		for (int i = MAX(l0, l1) - 1; i >= 0; i--) {
			for (int j = 0; j < w; j++) {
				ep2_dbl(r, r);
			}
			ep2_add(r, r, t[(w0[i] << w) + w1[i]]);
		}
		ep2_norm(r, r);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
		for (int i = 0; i < (1 << w); i++) {
			ep2_free(t0[i]);
			ep2_free(t1[i]);
		}
		for (int i = 0; i < (1 << EP_WIDTH); i++) {
			ep2_free(t[i]);
		}
	}
}
#endif

#if EP_SIM == INTER || !defined(STRIP)

void ep2_mul_sim_inter(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l) {
#if defined(EP_ENDOM)
	/* TODO. */
	//  if (ep_curve_is_kbltz()) {
	//      ep_mul_sim_kbltz(r, p, k, q, l, 0);
	//      return;
	//  }
#endif

#if defined(EP_PLAIN)
	ep2_mul_sim_plain(r, p, k, q, l, NULL);
#endif
}

#endif

#if EP_SIM == JOINT || !defined(STRIP)

void ep2_mul_sim_joint(ep2_t r, ep2_t p, bn_t k, ep2_t q, bn_t l) {
	ep2_t t[5];
	int u_i, len, offset;
	int8_t jsf[4 * (FP_BITS + 1)];
	int i;

	ep2_null(t[0]);
	ep2_null(t[1]);
	ep2_null(t[2]);
	ep2_null(t[3]);
	ep2_null(t[4]);

	TRY {
		for (i = 0; i < 5; i++) {
			ep2_new(t[i]);
		}

		ep2_set_infty(t[0]);
		ep2_copy(t[1], q);
		ep2_copy(t[2], p);
		ep2_add(t[3], p, q);
		ep2_sub(t[4], p, q);

		len = 4 * (FP_BITS + 1);
		bn_rec_jsf(jsf, &len, k, l);

		ep2_set_infty(r);

		i = bn_bits(k);
		offset = MAX(i, bn_bits(l)) + 1;
		for (i = len - 1; i >= 0; i--) {
			ep2_dbl(r, r);
			if (jsf[i] != 0 && jsf[i] == -jsf[i + offset]) {
				u_i = jsf[i] * 2 + jsf[i + offset];
				if (u_i < 0) {
					ep2_sub(r, r, t[4]);
				} else {
					ep2_add(r, r, t[4]);
				}
			} else {
				u_i = jsf[i] * 2 + jsf[i + offset];
				if (u_i < 0) {
					ep2_sub(r, r, t[-u_i]);
				} else {
					ep2_add(r, r, t[u_i]);
				}
			}
		}
		ep2_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < 5; i++) {
			ep2_free(t[i]);
		}
	}
}

#endif

void ep2_mul_sim_gen(ep2_t r, bn_t k, ep2_t q, bn_t l) {
	ep2_t gen;

	ep2_null(gen);

	TRY {
		ep2_new(gen);

		ep2_curve_get_gen(gen);
#if EP_FIX == LWNAF && defined(EP_PRECO)
		ep2_mul_sim_plain(r, gen, k, q, l, ep2_curve_get_tab());
#else
		ep2_mul_sim(r, gen, k, q, l);
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(gen);
	}
}
