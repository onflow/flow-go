/*
 * Copyright 2007-2009 RELIC Project
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
 * Implementation of simultaneous point multiplication on binary elliptic
 * curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EB_SIM == INTER || !defined(STRIP)

#if defined(EB_KBLTZ)

/**
 * Precomputes a table for a point multiplication on a Koblitz curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void table_init_koblitz(eb_t *t, eb_t p) {
	int u;

	if (eb_curve_opt_a() == EB_OPT_ZERO) {
		u = -1;
	} else {
		u = 1;
	}

	/* Prepare the precomputation table. */
	for (int i = 0; i < 1 << (EB_WIDTH - 2); i++) {
		eb_set_infty(t[i]);
		fb_set_bit(t[i]->z, 0, 1);
		t[i]->norm = 1;
	}

	eb_copy(t[0], p);

	/* The minimum table depth for WTNAF is 3. */
#if EB_WIDTH == 3
	eb_frb_tab(t[1], t[0]);
	if (u == 1) {
		eb_sub_tab(t[1], t[0], t[1]);
	} else {
		eb_add_tab(t[1], t[0], t[1]);
	}
#endif

#if EB_WIDTH == 4
	eb_frb_tab(t[3], t[0]);
	eb_frb_tab(t[3], t[3]);

	eb_sub_tab(t[1], t[3], p);
	eb_add_tab(t[2], t[3], p);
	eb_frb_tab(t[3], t[3]);

	if (u == 1) {
		eb_neg_tab(t[3], t[3]);
	}
	eb_sub_tab(t[3], t[3], p);
#endif

#if EB_WIDTH == 5
	eb_frb_tab(t[3], t[0]);
	eb_frb_tab(t[3], t[3]);

	eb_sub_tab(t[1], t[3], p);
	eb_add_tab(t[2], t[3], p);
	eb_frb_tab(t[3], t[3]);

	if (u == 1) {
		eb_neg_tab(t[3], t[3]);
	}
	eb_sub_tab(t[3], t[3], p);

	eb_frb_tab(t[4], t[2]);
	eb_frb_tab(t[4], t[4]);

	eb_sub_tab(t[7], t[4], t[2]);

	eb_neg_tab(t[4], t[4]);
	eb_sub_tab(t[5], t[4], p);
	eb_add_tab(t[6], t[4], p);

	eb_frb_tab(t[4], t[4]);
	if (u == -1) {
		eb_neg_tab(t[4], t[4]);
	}
	eb_add_tab(t[4], t[4], p);
#endif

#if EB_WIDTH == 6
	eb_frb_tab(t[0], t[0]);
	eb_frb_tab(t[0], t[0]);
	eb_neg_tab(t[14], t[0]);

	eb_sub_tab(t[13], t[14], p);
	eb_add_tab(t[14], t[14], p);

	eb_frb_tab(t[0], t[0]);
	if (u == -1) {
		eb_neg_tab(t[0], t[0]);
	}
	eb_sub_tab(t[11], t[0], p);
	eb_add_tab(t[12], t[0], p);

	eb_frb_tab(t[0], t[12]);
	eb_frb_tab(t[0], t[0]);
	eb_sub_tab(t[1], t[0], p);
	eb_add_tab(t[2], t[0], p);

	eb_add_tab(t[15], t[0], t[13]);

	eb_frb_tab(t[0], t[13]);
	eb_frb_tab(t[0], t[0]);
	eb_sub_tab(t[5], t[0], p);
	eb_add_tab(t[6], t[0], p);

	eb_neg_tab(t[8], t[0]);
	eb_add_tab(t[7], t[8], t[13]);
	eb_add_tab(t[8], t[8], t[14]);

	eb_frb_tab(t[0], t[0]);
	if (u == -1) {
		eb_neg_tab(t[0], t[0]);
	}
	eb_sub_tab(t[3], t[0], p);
	eb_add_tab(t[4], t[0], p);

	eb_frb_tab(t[0], t[1]);
	eb_frb_tab(t[0], t[0]);

	eb_neg_tab(t[9], t[0]);
	eb_sub_tab(t[9], t[9], p);

	eb_frb_tab(t[0], t[14]);
	eb_frb_tab(t[0], t[0]);
	eb_add_tab(t[10], t[0], p);

	eb_copy(t[0], p);
#endif
}

/**
 * Multiplies a binary elliptic curve point by an integer using the w-TNAF
 * method.
 *
 * @param[out] r 				- the result.
 * @param[in] p					- the point to multiply.
 * @param[in] k					- the integer.
 * @param[in] w					- the window size.
 */
static void eb_mul_sim_kbltz(eb_t r, eb_t p, bn_t k, eb_t q, bn_t l, int gen) {
	int l0, l1, len, i, n0, n1, u, w;
	signed char tnaf0[FB_BITS + 8], *t0;
	signed char tnaf1[FB_BITS + 8], *t1;
	eb_t table0[1 << (EB_WIDTH - 2)] = { NULL };
	eb_t table1[1 << (EB_WIDTH - 2)] = { NULL };
	eb_t *t = NULL;

	/* Compute the w-TNAF representation of k. */
	if (eb_curve_opt_a() == EB_OPT_ZERO) {
		u = -1;
	} else {
		u = 1;
	}

	if (gen == 1) {
#if defined(EB_PRECO)
		t = eb_curve_get_tab();
#endif
	} else {
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_new(table0[i]);
			eb_set_infty(table0[i]);
			fb_set_bit(table0[i]->z, 0, 1);
			table0[i]->norm = 1;
		}
		table_init_koblitz(table0, p);
		t = table0;
	}

	/* Prepare the precomputation table. */
	for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_new(table1[i]);
	}
	/* Compute the precomputation table. */
	table_init_koblitz(table1, q);

	/* Compute the w-TNAF representation of k. */
	if (gen) {
		w = EB_DEPTH;
	} else {
		w = EB_WIDTH;
	}
	bn_rec_tnaf(tnaf0, &l0, k, eb_curve_get_vm(), eb_curve_get_s0(),
			eb_curve_get_s1(), u, FB_BITS, w);

	bn_rec_tnaf(tnaf1, &l1, l, eb_curve_get_vm(), eb_curve_get_s0(),
			eb_curve_get_s1(), u, FB_BITS, EB_WIDTH);

	len = MAX(l0, l1);
	t0 = tnaf0 + len - 1;
	t1 = tnaf1 + len - 1;
	for (i = l0; i < len; i++)
		tnaf0[i] = 0;
	for (i = l1; i < len; i++)
		tnaf1[i] = 0;

	t0 = tnaf0 + len - 1;
	t1 = tnaf1 + len - 1;
	eb_set_infty(r);
	for (i = len - 1; i >= 0; i--, t0--, t1--) {
		eb_frb(r, r);

		n0 = *t0;
		n1 = *t1;
		if (n0 > 0) {
			eb_add(r, r, t[n0 / 2]);
		}
		if (n0 < 0) {
			eb_sub(r, r, t[-n0 / 2]);
		}
		if (n1 > 0) {
			eb_add(r, r, table1[n1 / 2]);
		}
		if (n1 < 0) {
			eb_sub(r, r, table1[-n1 / 2]);
		}
	}
	/* Convert r to affine coordinates. */
	eb_norm(r, r);
}

#endif /* EB_KBLTZ */

#if defined(EB_ORDIN) || defined(EB_SUPER)

/**
 * Precomputes a table for a point multiplication on an ordinary curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
void table_init_ordin(eb_t *t, eb_t p) {
	int i;

	for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_set_infty(t[i]);
		fb_set_bit(t[i]->z, 0, 1);
		t[i]->norm = 1;
	}

	eb_dbl_tab(t[0], p);

#if EB_WIDTH > 2
	eb_add_tab(t[1], t[0], p);
	for (i = 2; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_add_tab(t[i], t[i - 1], t[0]);
	}
#endif

	eb_copy(t[0], p);
}

static void eb_mul_sim_ordin(eb_t r, eb_t p, bn_t k, eb_t q, bn_t l, int gen) {
	int len, l0, l1, i, n0, n1, w;
	signed char naf0[FB_BITS + 1], naf1[FB_BITS + 1], *t0, *t1;
	eb_t table0[1 << (EB_WIDTH - 2)] = { NULL };
	eb_t table1[1 << (EB_WIDTH - 2)] = { NULL };
	eb_t *t = NULL;

	if (gen) {
#if defined(EB_PRECO)
		t = eb_curve_get_tab();
#endif
	} else {
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_new(table0[i]);
		}
		table_init_ordin(table0, p);
		t = table0;
	}

	/* Prepare the precomputation table. */
	for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_new(table1[i]);
	}
	/* Compute the precomputation table. */
	table_init_ordin(table1, q);

	/* Compute the w-TNAF representation of k. */
	if (gen) {
		w = EB_DEPTH;
	} else {
		w = EB_WIDTH;
	}
	bn_rec_naf(naf0, &l0, k, w);
	bn_rec_naf(naf1, &l1, l, EB_WIDTH);

	len = MAX(l0, l1);
	t0 = naf0 + len - 1;
	t1 = naf1 + len - 1;
	for (i = l0; i < len; i++)
		naf0[i] = 0;
	for (i = l1; i < len; i++)
		naf1[i] = 0;

	eb_set_infty(r);
	for (i = len - 1; i >= 0; i--, t0--, t1--) {
		eb_dbl(r, r);

		n0 = *t0;
		n1 = *t1;
		if (n0 > 0) {
			eb_add(r, r, t[n0 / 2]);
		}
		if (n0 < 0) {
			eb_sub(r, r, t[-n0 / 2]);
		}
		if (n1 > 0) {
			eb_add(r, r, table1[n1 / 2]);
		}
		if (n1 < 0) {
			eb_sub(r, r, table1[-n1 / 2]);
		}
	}
	/* Convert r to affine coordinates. */
	eb_norm(r, r);

	/* Free the precomputation table. */
	for (i = 0; i < 1 << (EB_WIDTH - 2); i++) {
		eb_free(table0[i]);
		eb_free(table1[i]);
	}
}

#endif /* EB_ORDIN || EB_SUPER */

#endif /* EB_SIM == INTER */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EB_SIM == BASIC || !defined(STRIP)

void eb_mul_sim_basic(eb_t r, eb_t p, bn_t k, eb_t q, bn_t l) {
	eb_t t = NULL;

	TRY {
		eb_new(t);
		eb_mul(t, q, l);
		eb_mul(r, p, k);
		eb_add(t, t, r);
		eb_norm(r, t);

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(t);
	}
}

#endif

#if EB_SIM == TRICK || !defined(STRIP)

void eb_mul_sim_trick(eb_t r, eb_t p, bn_t k, eb_t q, bn_t l) {
	eb_t t0[1 << (EB_WIDTH / 2)] = { NULL };
	eb_t t1[1 << (EB_WIDTH / 2)] = { NULL };
	eb_t t[1 << EB_WIDTH] = { NULL };
	int d, l0, l1, w;
	unsigned char w0[FB_BITS + 1], w1[FB_BITS + 1];

	w = EB_WIDTH / 2;

	d = bn_bits(eb_curve_get_ord());
	d = ((d % w) == 0 ? (d / w) : (d / w) + 1);

	TRY {
		for (int i = 0; i < (1 << w); i++) {
			eb_new(t0[i]);
			eb_new(t1[i]);
		}
		for (int i = 0; i < (1 << EB_WIDTH); i++) {
			eb_new(t[i]);
		}

		eb_set_infty(t0[0]);
		for (int i = 1; i < (1 << w); i++) {
			eb_add_tab(t0[i], t0[i - 1], p);
		}

		eb_set_infty(t1[0]);
		for (int i = 1; i < (1 << w); i++) {
			eb_add_tab(t1[i], t1[i - 1], q);
		}

		for (int i = 0; i < (1 << w); i++) {
			for (int j = 0; j < (1 << w); j++) {
				eb_add_tab(t[(i << w) + j], t0[i], t1[j]);
			}
		}

		bn_rec_win(w0, &l0, k, w);
		bn_rec_win(w1, &l1, l, w);

		for (int i = l0; i < l1; i++) {
			w0[i] = 0;
		}
		for (int i = l1; i < l0; i++) {
			w1[i] = 0;
		}

		eb_set_infty(r);
		for (int i = MAX(l0, l1) - 1; i >= 0; i--) {
			for (int j = 0; j < w; j++) {
				eb_dbl(r, r);
			}
			eb_add(r, r, t[(w0[i] << w) + w1[i]]);
		}
		eb_norm(r, r);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (int i = 0; i < (1 << w); i++) {
			eb_free(t0[i]);
			eb_free(t1[i]);
		}
		for (int i = 0; i < (1 << EB_WIDTH); i++) {
			eb_free(t[i]);
		}
	}
}
#endif

#if EB_SIM == INTER || !defined(STRIP)

void eb_mul_sim_inter(eb_t r, eb_t p, bn_t k, eb_t q, bn_t l) {
#if defined(EB_KBLTZ)
	if (eb_curve_is_kbltz()) {
		eb_mul_sim_kbltz(r, p, k, q, l, 0);
		return;
	}
#endif

#if defined(EB_ORDIN)
	eb_mul_sim_ordin(r, p, k, q, l, 0);
#endif
}

#endif

#if EB_SIM == JOINT || !defined(STRIP)

void eb_mul_sim_joint(eb_t r, eb_t p, bn_t k, eb_t q, bn_t l) {
	eb_t t[5] = { NULL };
	int u_i, len, offset;
	signed char jsf[2 * (FB_BITS + 1)];

	TRY {
		for (int i = 0; i < 5; i++) {
			eb_new(t[i]);
		}

		eb_set_infty(t[0]);
		eb_copy(t[1], q);
		eb_copy(t[2], p);
		eb_add_tab(t[3], p, q);
		eb_sub_tab(t[4], p, q);

		bn_rec_jsf(jsf, &len, k, l);

		eb_set_infty(r);

		offset = MAX(bn_bits(k),bn_bits(l)) + 1;
		for (int i = len - 1; i >= 0; i--) {
			eb_dbl(r, r);
			if (jsf[i] != 0 && jsf[i] == -jsf[i + offset]) {
				u_i = jsf[i] * 2 + jsf[i + offset];
				if (u_i < 0) {
					eb_sub(r, r, t[4]);
				} else {
					eb_add(r, r, t[4]);
				}
			} else {
				u_i = jsf[i] * 2 + jsf[i + offset];
				if (u_i < 0) {
					eb_sub(r, r, t[-u_i]);
				} else {
					eb_add(r, r, t[u_i]);
				}
			}
		}
		eb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (int i = 0; i < 5; i++) {
			eb_free(t[i]);
		}
	}
}

#endif

void eb_mul_sim_gen(eb_t r, bn_t k, eb_t q, bn_t l) {
#if defined(EB_KBLTZ)
#if EB_FIX == WTNAF && defined(EB_PRECO)
	if (eb_curve_is_kbltz()) {
		eb_mul_sim_kbltz(r, eb_curve_get_gen(), k, q, l, 1);
		return;
	}
#else
	if (eb_curve_is_kbltz()) {
		eb_mul_sim(r, eb_curve_get_gen(), k, q, l);
		return;
	}
#endif
#endif

#if defined(EB_ORDIN)
#if EB_FIX == WTNAF && defined(EB_PRECO)
	eb_mul_sim_ordin(r, eb_curve_get_gen(), k, q, l, 1);
#else
	eb_mul_sim(r, eb_curve_get_gen(), k, q, l);
#endif
#endif
}
