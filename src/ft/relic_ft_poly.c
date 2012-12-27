/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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

#if FT_CRT == QUICK || !defined(STRIP)

/**
 * Precomputes the cube root of z and its square.
 */
void find_crz() {
	ctx_t *ctx = core_get();
	ft_set_trit(ctx->ft_crz, 1, 1);

	for (int i = 1; i < FT_TRITS; i++) {
		ft_cub(ctx->ft_crz, ctx->ft_crz);
	}
	for (int i = 0; i < FT_TRITS; i++) {
		if (ft_get_trit(ctx->ft_crz, i) == 1) {
			ctx->crz[ctx->crz_len++] = i;
		}
		if (ft_get_trit(ctx->ft_crz, i) == 2) {
			ctx->crz[ctx->crz_len++] = -i;
		}
		if (ctx->crz_len == MAX_TERMS) {
			ctx->crz_len = 0;
			break;
		}
	}
	ft_mul(ctx->ft_srz, ctx->ft_crz, ctx->ft_crz);

	for (int i = 0; i < FT_TRITS; i++) {
		if (ft_get_trit(ctx->ft_srz, i) == 1) {
			ctx->srz[ctx->srz_len++] = i;
		}
		if (ft_get_trit(ctx->ft_srz, i) == 2) {
			ctx->srz[ctx->srz_len++] = -i;
		}
		if (ctx->srz_len == MAX_TERMS) {
			ctx->srz_len = 0;
			break;
		}
	}
#ifdef FT_PRECO
	for (int i = 0; i <= 255; i++) {
		//ft_mul_dig(ctx->ft_tab_crz[i], ctx->ft_crz, i);
	}
#endif
}

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_poly_init(void) {
	ctx_t *ctx = core_get();
	ft_zero(ctx->ft_poly);
	ctx->ft_pa = ctx->ft_pb = ctx->ft_pc = 0;
	ctx->ft_na = ctx->ft_nb = ctx->ft_nc = -1;
}

void ft_poly_clean(void) {
}

dig_t *ft_poly_get(void) {
	return core_get()->ft_poly;
}

void ft_poly_set(ft_t f) {
	ft_copy(core_get()->ft_poly, f);

#if FT_CRT == QUICK || !defined(STRIP)
	find_crz();
#endif

}

void ft_poly_add(ft_t c, ft_t a) {
	ctx_t *ctx = core_get();
	dig_t t;
	dig_t *c_l = c, *c_h = c + FT_DIGS / 2;
	dig_t *f_l = ctx->ft_poly, *f_h = ctx->ft_poly + FT_DIGS / 2;

	if (c != a) {
		ft_copy(c, a);
	}

	/* Polynomial is sparse. */
	if (ctx->ft_pa != 0 && ctx->ft_pb != 0) {
		t = (c_l[FT_DIGS / 2 - 1] | c_h[FT_DIGS / 2 - 1]) &
				(f_l[FT_DIGS / 2 - 1] | f_h[FT_DIGS / 2 - 1]);
		c_l[FT_DIGS / 2 - 1] = t ^
				(c_l[FT_DIGS / 2 - 1] | f_l[FT_DIGS / 2 - 1]);
		c_h[FT_DIGS / 2 - 1] = t ^
				(c_h[FT_DIGS / 2 - 1] | f_h[FT_DIGS / 2 - 1]);

		/* At least it is a trinomial. */
		if (ctx->ft_na != FT_DIGS / 2 - 1) {
			t = (c_l[ctx->ft_na] | c_h[ctx->ft_na]) & (f_l[ctx->ft_na] | f_h[ctx->ft_na]);
			c_l[ctx->ft_na] = t ^ (c_l[ctx->ft_na] | f_l[ctx->ft_na]);
			c_h[ctx->ft_na] = t ^ (c_h[ctx->ft_na] | f_h[ctx->ft_na]);
		}
		if (ctx->ft_nb != ctx->ft_na) {
			t = (c_l[ctx->ft_nb] | c_h[ctx->ft_nb]) & (f_l[ctx->ft_nb] | f_h[ctx->ft_nb]);
			c_l[ctx->ft_nb] = t ^ (c_l[ctx->ft_nb] | f_l[ctx->ft_nb]);
			c_h[ctx->ft_nb] = t ^ (c_h[ctx->ft_nb] | f_h[ctx->ft_nb]);
		}

		/* Maybe a pentanomial? */
		if (ctx->ft_pc != 0 && ctx->ft_pd != 0) {
			if (ctx->ft_nc != ctx->ft_na && ctx->ft_nc != ctx->ft_nb) {
				t = (c_l[ctx->ft_nc] | c_h[ctx->ft_nc]) & (f_l[ctx->ft_nc] | f_h[ctx->ft_nc]);
				c_l[ctx->ft_nc] = t ^ (c_l[ctx->ft_nc] | f_l[ctx->ft_nc]);
				c_h[ctx->ft_nc] = t ^ (c_h[ctx->ft_nc] | f_h[ctx->ft_nc]);
			}
			if (ctx->ft_na != 0 && ctx->ft_nb != 0 && ctx->ft_nc != 0) {
				t = (c_l[0] | c_h[0]) & (f_l[0] | f_h[0]);
				c_l[0] = t ^ (c_l[0] | f_l[0]);
				c_h[0] = t ^ (c_h[0] | f_h[0]);
			}
		}
	} else {
		/* Polynomial is dense. */
		ft_add(c, a, ctx->ft_poly);
	}
}

void ft_poly_sub(ft_t c, ft_t a) {
	ctx_t *ctx = core_get();
	dig_t t0, t1;
	dig_t *c_l = c, *c_h = c + FT_DIGS / 2;
	dig_t *f_l = ctx->ft_poly, *f_h = ctx->ft_poly + FT_DIGS / 2;

	if (c != a) {
		ft_copy(c, a);
	}

	/* Polynomial is sparse. */
	if (ctx->ft_pa != 0 && ctx->ft_pb != 0) {
		t0 = (c_l[FT_DIGS / 2 - 1] | c_h[FT_DIGS / 2 - 1]) &
				(f_l[FT_DIGS / 2 - 1] | f_h[FT_DIGS / 2 - 1]);
		t1 = (c_h[FT_DIGS / 2 - 1] | f_l[FT_DIGS / 2 - 1]);
		c_l[FT_DIGS / 2 - 1] =
				t0 ^ (c_l[FT_DIGS / 2 - 1] | f_h[FT_DIGS / 2 - 1]);
		c_h[FT_DIGS / 2 - 1] = t0 ^ t1;

		/* At least it is a trinomial. */
		if (ctx->ft_na != FT_DIGS / 2 - 1) {
			t0 = (c_l[ctx->ft_na] | c_h[ctx->ft_na]) & (f_l[ctx->ft_na] | f_h[ctx->ft_na]);
			t1 = (c_h[ctx->ft_na] | f_l[ctx->ft_na]);
			c_l[ctx->ft_na] = t0 ^ (c_l[ctx->ft_na] | f_h[ctx->ft_na]);
			c_h[ctx->ft_na] = t0 ^ t1;
		}
		if (ctx->ft_nb != ctx->ft_na) {
			t0 = (c_l[ctx->ft_nb] | c_h[ctx->ft_nb]) & (f_l[ctx->ft_nb] | f_h[ctx->ft_nb]);
			t1 = (c_h[ctx->ft_nb] | f_l[ctx->ft_nb]);
			c_l[ctx->ft_nb] = t0 ^ (c_l[ctx->ft_nb] | f_h[ctx->ft_nb]);
			c_h[ctx->ft_nb] = t0 ^ t1;
		}

		/* Maybe a pentanomial? */
		if (ctx->ft_pc != 0 && ctx->ft_pd != 0) {
			if (ctx->ft_nc != ctx->ft_na && ctx->ft_nc != ctx->ft_nb) {
				t0 = (c_l[ctx->ft_nc] | c_h[ctx->ft_nc]) & (f_l[ctx->ft_nc] | f_h[ctx->ft_nc]);
				t1 = (c_h[ctx->ft_nc] | f_l[ctx->ft_nc]);
				c_l[ctx->ft_nc] = t0 ^ (c_l[ctx->ft_nc] | f_h[ctx->ft_nc]);
				c_h[ctx->ft_nc] = t0 ^ t1;
			}
			if (ctx->ft_na != 0 && ctx->ft_nb != 0 && ctx->ft_nc != 0) {
				t0 = (c_l[0] | c_h[0]) & (f_l[0] | f_h[0]);
				t1 = (c_h[0] | f_l[0]);
				c_l[0] = t0 ^ (c_l[0] | f_h[0]);
				c_h[0] = t0 ^ t1;
			}
		}
	} else {
		/* Polynomial is dense. */
		ft_add(c, a, ctx->ft_poly);
	}
}

void ft_poly_set_trino(int a, int b) {
	ft_t f;
	ctx_t *ctx = core_get();

	ft_null(f);

	TRY {
		ft_new(f);

		ctx->ft_pa = a;
		ctx->ft_pb = b;
		ctx->ft_pc = ctx->ft_pd = 0;

		if (ctx->ft_pa > 0) {
			ctx->ft_na = ctx->ft_pa >> FT_DIG_LOG;
		} else {
			ctx->ft_na = -ctx->ft_pa >> FT_DIG_LOG;
		}
		if (ctx->ft_pb > 0) {
			ctx->ft_nb = ctx->ft_pb >> FT_DIG_LOG;
		} else {
			ctx->ft_nb = -ctx->ft_pb >> FT_DIG_LOG;
		}
		ctx->ft_nc = -1;

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
	ctx_t *ctx = core_get();

	ft_null(f);

	TRY {
		ft_new(f);

		ctx->ft_pa = a;
		ctx->ft_pb = b;
		ctx->ft_pc = c;
		ctx->ft_pd = d;

		if (ctx->ft_pa > 0) {
			ctx->ft_na = ctx->ft_pa >> FT_DIG_LOG;
		} else {
			ctx->ft_na = -ctx->ft_pa >> FT_DIG_LOG;
		}
		if (ctx->ft_pb > 0) {
			ctx->ft_nb = ctx->ft_pb >> FT_DIG_LOG;
		} else {
			ctx->ft_nb = -ctx->ft_pb >> FT_DIG_LOG;
		}
		if (ctx->ft_pc > 0) {
			ctx->ft_nc = ctx->ft_pc >> FT_DIG_LOG;
		} else {
			ctx->ft_nc = -ctx->ft_pc >> FT_DIG_LOG;
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
	return core_get()->ft_crz;
#else
	return NULL;
#endif
}

int *ft_poly_get_crz_sps(int *len) {
#if FT_CRT == QUICK || !defined(STRIP)
	ctx_t *ctx = core_get();

	if (ctx->crz_len > 0 && ctx->crz_len < MAX_TERMS ) {
		if (len != NULL) {
			*len = ctx->crz_len;
		}
		return ctx->crz;
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
	return core_get()->ft_srz;
#else
	return NULL;
#endif
}

int *ft_poly_get_srz_sps(int *len) {
#if FT_CRT == QUICK || !defined(STRIP)
	ctx_t *ctx = core_get();

	if (ctx->srz_len > 0 && ctx->srz_len < MAX_TERMS ) {
		if (len != NULL) {
			*len = ctx->srz_len;
		}
		return ctx->srz;
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
#if FT_CRT == QUICK || !defined(STRIP)

#ifdef FT_PRECO
	return core_get()->ft_tab_crz[i];
#else
	return NULL;
#endif

#else
	return NULL;
#endif
}

dig_t *ft_poly_get_tab_srz(int i) {
#if FT_CRT == QUICK || !defined(STRIP)

#ifdef FT_PRECO
	return core_get()->ft_tab_srz[i];
#else
	return NULL;
#endif

#else
	return NULL;
#endif
}

void ft_poly_get_rdc(int *a, int *b, int *c, int *d) {
	ctx_t *ctx = core_get();
	*a = ctx->ft_pa;
	*b = ctx->ft_pb;
	*c = ctx->ft_pc;
	*d = ctx->ft_pd;
}
