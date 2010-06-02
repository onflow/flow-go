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
 * Implementation of the binary field modulus manipulation.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_dv.h"
#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Prime modulus.
 */
static ft_st poly;

/**
 * Trinomial or pentanomial non-zero coefficients.
 */
static int poly_a, poly_b, poly_c, poly_d;

/**
 * Positions of the non-null coefficients on trinomials and pentanomials.
 */
static int pos_a, pos_b, pos_c;

#if FT_CRT == QUICK || !defined(STRIP)

/**
 * Maximum number of exponents to describe a sparse polynomial.
 */
#define MAX_EXPS		10

/**
 * Non-zero bits of special form prime.
 */
static int crz[MAX_EXPS + 1] = { 0 };

/**
 * Number of bits of special form prime.
 */
static int crz_len = 0;

/**
 * Cube root of z.
 */
static ft_st ft_crz;

/**
 * Non-zero bits of special form prime.
 */
static int srz[MAX_EXPS + 1] = { 0 };

/**
 * Number of bits of special form prime.
 */
static int srz_len = 0;

/**
 * Square of cube root of z.
 */
static ft_st ft_srz;

#ifdef FT_PRECO
/**
 * Multiplication table for the z^(1/3).
 */
static ft_st ft_tab_crz[256];

/**
 * Multiplication table for the z^(2/3).
 */
static ft_st ft_tab_srz[256];

#endif

/**
 * Precomputes the cube root of z and its square.
 */
void find_crz() {
	ft_set_trit(ft_crz, 1, 1);

	for (int i = 1; i < FT_TRITS; i++) {
		ft_cub(ft_crz, ft_crz);
	}
	for (int i = 0; i < FT_TRITS; i++) {
		if (ft_get_trit(ft_crz, i) == 1) {
			crz[crz_len++] = i;
		}
		if (ft_get_trit(ft_crz, i) == 2) {
			crz[crz_len++] = -i;
		}
		if (crz_len == MAX_EXPS) {
			crz_len = 0;
			break;
		}
	}
	ft_mul(ft_srz, ft_crz, ft_crz);

	for (int i = 0; i < FT_TRITS; i++) {
		if (ft_get_trit(ft_srz, i) == 1) {
			srz[srz_len++] = i;
		}
		if (ft_get_trit(ft_srz, i) == 2) {
			srz[srz_len++] = -i;
		}
		if (srz_len == MAX_EXPS) {
			srz_len = 0;
			break;
		}
	}
#ifdef FT_PRECO
	for (int i = 0; i <= 255; i++) {
		//ft_mul_dig(ft_tab_crz[i], ft_crz, i);
	}
#endif
}

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_poly_init(void) {
	ft_zero(poly);
	poly_a = poly_b = poly_c = 0;
	pos_a = pos_b = pos_c = -1;
}

void ft_poly_clean(void) {
}

dig_t *ft_poly_get(void) {
	return poly;
}

void ft_poly_set(ft_t f) {
	ft_copy(poly, f);

#if FT_CRT == QUICK || !defined(STRIP)
	find_crz();
#endif

}

void ft_poly_add(ft_t c, ft_t a) {
	dig_t t;
	dig_t *c_l = c, *c_h = c + FT_DIGS / 2;
	dig_t *f_l = poly, *f_h = poly + FT_DIGS / 2;

	if (c != a) {
		ft_copy(c, a);
	}

	/* Polynomial is sparse. */
	if (poly_a != 0 && poly_b != 0) {
		t = (c_l[FT_DIGS / 2 - 1] | c_h[FT_DIGS / 2 - 1]) &
				(f_l[FT_DIGS / 2 - 1] | f_h[FT_DIGS / 2 - 1]);
		c_l[FT_DIGS / 2 - 1] = t ^
				(c_l[FT_DIGS / 2 - 1] | f_l[FT_DIGS / 2 - 1]);
		c_h[FT_DIGS / 2 - 1] = t ^
				(c_h[FT_DIGS / 2 - 1] | f_h[FT_DIGS / 2 - 1]);

		/* At least it is a trinomial. */
		if (pos_a != FT_DIGS / 2 - 1) {
			t = (c_l[pos_a] | c_h[pos_a]) & (f_l[pos_a] | f_h[pos_a]);
			c_l[pos_a] = t ^ (c_l[pos_a] | f_l[pos_a]);
			c_h[pos_a] = t ^ (c_h[pos_a] | f_h[pos_a]);
		}
		if (pos_b != pos_a) {
			t = (c_l[pos_b] | c_h[pos_b]) & (f_l[pos_b] | f_h[pos_b]);
			c_l[pos_b] = t ^ (c_l[pos_b] | f_l[pos_b]);
			c_h[pos_b] = t ^ (c_h[pos_b] | f_h[pos_b]);
		}

		/* Maybe a pentanomial? */
		if (poly_c != 0 && poly_d != 0) {
			if (pos_c != pos_a && pos_c != pos_b) {
				t = (c_l[pos_c] | c_h[pos_c]) & (f_l[pos_c] | f_h[pos_c]);
				c_l[pos_c] = t ^ (c_l[pos_c] | f_l[pos_c]);
				c_h[pos_c] = t ^ (c_h[pos_c] | f_h[pos_c]);
			}
			if (pos_a != 0 && pos_b != 0 && pos_c != 0) {
				t = (c_l[0] | c_h[0]) & (f_l[0] | f_h[0]);
				c_l[0] = t ^ (c_l[0] | f_l[0]);
				c_h[0] = t ^ (c_h[0] | f_h[0]);
			}
		}
	} else {
		/* Polynomial is dense. */
		ft_add(c, a, poly);
	}
}

void ft_poly_sub(ft_t c, ft_t a) {
	dig_t t0, t1;
	dig_t *c_l = c, *c_h = c + FT_DIGS / 2;
	dig_t *f_l = poly, *f_h = poly + FT_DIGS / 2;

	if (c != a) {
		ft_copy(c, a);
	}

	/* Polynomial is sparse. */
	if (poly_a != 0 && poly_b != 0) {
		t0 = (c_l[FT_DIGS / 2 - 1] | c_h[FT_DIGS / 2 - 1]) &
				(f_l[FT_DIGS / 2 - 1] | f_h[FT_DIGS / 2 - 1]);
		t1 = (c_h[FT_DIGS / 2 - 1] | f_l[FT_DIGS / 2 - 1]);
		c_l[FT_DIGS / 2 - 1] =
				t0 ^ (c_l[FT_DIGS / 2 - 1] | f_h[FT_DIGS / 2 - 1]);
		c_h[FT_DIGS / 2 - 1] = t0 ^ t1;

		/* At least it is a trinomial. */
		if (pos_a != FT_DIGS / 2 - 1) {
			t0 = (c_l[pos_a] | c_h[pos_a]) & (f_l[pos_a] | f_h[pos_a]);
			t1 = (c_h[pos_a] | f_l[pos_a]);
			c_l[pos_a] = t0 ^ (c_l[pos_a] | f_h[pos_a]);
			c_h[pos_a] = t0 ^ t1;
		}
		if (pos_b != pos_a) {
			t0 = (c_l[pos_b] | c_h[pos_b]) & (f_l[pos_b] | f_h[pos_b]);
			t1 = (c_h[pos_b] | f_l[pos_b]);
			c_l[pos_b] = t0 ^ (c_l[pos_b] | f_h[pos_b]);
			c_h[pos_b] = t0 ^ t1;
		}

		/* Maybe a pentanomial? */
		if (poly_c != 0 && poly_d != 0) {
			if (pos_c != pos_a && pos_c != pos_b) {
				t0 = (c_l[pos_c] | c_h[pos_c]) & (f_l[pos_c] | f_h[pos_c]);
				t1 = (c_h[pos_c] | f_l[pos_c]);
				c_l[pos_c] = t0 ^ (c_l[pos_c] | f_h[pos_c]);
				c_h[pos_c] = t0 ^ t1;
			}
			if (pos_a != 0 && pos_b != 0 && pos_c != 0) {
				t0 = (c_l[0] | c_h[0]) & (f_l[0] | f_h[0]);
				t1 = (c_h[0] | f_l[0]);
				c_l[0] = t0 ^ (c_l[0] | f_h[0]);
				c_h[0] = t0 ^ t1;
			}
		}
	} else {
		/* Polynomial is dense. */
		ft_add(c, a, poly);
	}
}

void ft_poly_set_trino(int a, int b) {
	ft_t f;

	ft_null(f);

	TRY {
		ft_new(f);

		poly_a = a;
		poly_b = b;
		poly_c = poly_d = 0;

		if (poly_a > 0) {
			pos_a = poly_a >> FT_DIG_LOG;
		} else {
			pos_a = -poly_a >> FT_DIG_LOG;
		}
		if (poly_b > 0) {
			pos_b = poly_b >> FT_DIG_LOG;
		} else {
			pos_b = -poly_b >> FT_DIG_LOG;
		}
		pos_c = -1;

		ft_zero(f);
		ft_set_trit(f, FT_TRITS, 1);
		if (a > 0) {
			ft_set_trit(f, a, 1);
		} else {
			ft_set_trit(f, -a, 2);
		}
		if (b > 0) {
			ft_set_trit(f, 0, 1);
		} else {
			ft_set_trit(f, 0, 2);
		}
		ft_poly_set(f);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ft_free(f);
	}
}

void ft_poly_set_penta(int a, int b, int c, int d) {
	ft_t f;

	ft_null(f);

	TRY {
		ft_new(f);

		poly_a = a;
		poly_b = b;
		poly_c = c;
		poly_d = d;

		if (poly_a > 0) {
			pos_a = poly_a >> FT_DIG_LOG;
		} else {
			pos_a = -poly_a >> FT_DIG_LOG;
		}
		if (poly_b > 0) {
			pos_b = poly_b >> FT_DIG_LOG;
		} else {
			pos_b = -poly_b >> FT_DIG_LOG;
		}
		if (poly_c > 0) {
			pos_c = poly_c >> FT_DIG_LOG;
		} else {
			pos_c = -poly_c >> FT_DIG_LOG;
		}

		ft_zero(f);
		ft_set_trit(f, FT_TRITS, 1);
		if (a > 0) {
			ft_set_trit(f, a, 1);
		} else {
			ft_set_trit(f, -a, 2);
		}
		if (b > 0) {
			ft_set_trit(f, b, 1);
		} else {
			ft_set_trit(f, -b, 2);
		}
		if (c > 0) {
			ft_set_trit(f, c, 1);
		} else {
			ft_set_trit(f, -c, 2);
		}
		if (d > 0) {
			ft_set_trit(f, 0, 1);
		} else {
			ft_set_trit(f, 0, 2);
		}
		ft_poly_set(f);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ft_free(f);
	}
}

dig_t *ft_poly_get_crz(void) {
#if FT_CRT == QUICK || !defined(STRIP)
	return ft_crz;
#else
	return NULL;
#endif
}

int *ft_poly_get_crz_spars(int *len) {
#if FT_CRT == QUICK || !defined(STRIP)
	if (crz_len > 0 && crz_len < MAX_EXPS ) {
		if (len != NULL) {
			*len = crz_len;
		}
		return crz;
	} else {
		if (len != NULL) {
			*len = 0;
		}
		return NULL;
	}
#else
	*len = 0;
	return NULL;
#endif
}

dig_t *ft_poly_get_srz(void) {
#if FT_CRT == QUICK || !defined(STRIP)
	return ft_srz;
#else
	return NULL;
#endif
}

int *ft_poly_get_srz_spars(int *len) {
#if FT_CRT == QUICK || !defined(STRIP)
	if (srz_len > 0 && srz_len < MAX_EXPS ) {
		if (len != NULL) {
			*len = srz_len;
		}
		return srz;
	} else {
		if (len != NULL) {
			*len = 0;
		}
		return NULL;
	}
#else
	*len = 0;
	return NULL;
#endif
}

dig_t *ft_poly_get_tab_crz(int i) {
#if FB_SRT == QUICK || !defined(STRIP)

#ifdef FT_PRECO
	return ft_tab_crz[i];
#else
	return NULL;
#endif

#else
	return NULL;
#endif
}

dig_t *ft_poly_get_tab_srz(int i) {
#if FB_SRT == QUICK || !defined(STRIP)

#ifdef FT_PRECO
	return ft_tab_srz[i];
#else
	return NULL;
#endif

#else
	return NULL;
#endif
}

void ft_poly_get_rdc(int *a, int *b, int *c, int *d) {
	*a = poly_a;
	*b = poly_b;
	*c = poly_c;
	*d = poly_d;
}
