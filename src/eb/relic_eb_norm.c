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
 * Implementation of the point normalization on binary elliptic curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EB_ADD == PROJC || defined(EB_MIXED)

#if defined(EB_ORDIN) || defined(EB_KBLTZ)

/**
 * Normalizes a point represented in projective coordinates.
 *
 * @param[out] r		- the result.
 * @param[in] p			- the point to normalize.
 * @param[in] flag		- if the Z coordinate is already inverted.
 */
static void eb_norm_ordin(eb_t r, eb_t p, int flag) {
	if (!p->norm) {
		if (flag) {
			fb_copy(r->z, p->z);
		} else {
			fb_inv(r->z, p->z);
		}
		fb_mul(r->x, p->x, r->z);
		fb_sqr(r->z, r->z);
		fb_mul(r->y, p->y, r->z);
		fb_set_dig(r->z, 1);
	}

	r->norm = 1;
}

#endif /* EB_ORDIN || EB_KBLTZ */

#if defined(EB_SUPER)

/**
 * Normalizes a point represented in projective coordinates.
 *
 * @param r			- the result.
 * @param p			- the point to normalize.
 */
static void eb_norm_super(eb_t r, eb_t p, int flag) {
	if (!p->norm) {
		if (flag) {
			fb_copy(r->z, p->z);
		} else {
			fb_inv(r->z, p->z);
		}
		fb_mul(r->x, p->x, r->z);
		fb_mul(r->y, p->y, r->z);
		fb_set_dig(r->z, 1);
	}

	r->norm = 1;
}

#endif /* EB_SUPER */

#endif /* EB_ADD == PROJC || EB_MIXED */

/**
 * Normalizes a point represented in lambda-coordinates.
 *
 * @param[out] r		- the result.
 * @param[in] p			- the point to normalize.
 */
static void eb_norm_halve(eb_t r, eb_t p) {
	fb_t t0;

	fb_null(t0);

	TRY {
		fb_new(t0);

		fb_sqr(t0, p->x);
		fb_mul(r->y, p->x, p->y);
		fb_add(r->y, r->y, t0);
		fb_copy(r->x, p->x);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
	}

	r->norm = 1;
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void eb_norm(eb_t r, eb_t p) {
	if (eb_is_infty(p)) {
		eb_set_infty(r);
		return;
	}

	if (p->norm == 1) {
		/* If the point is represented in affine coordinates, we just copy it. */
		eb_copy(r, p);
		return;
	}

	if (p->norm == 2) {
		eb_norm_halve(r, p);
		return;
	}
#if EB_ADD == PROJC || !defined(STRIP)

#if defined(EB_SUPER)
	if (eb_curve_is_super()) {
		eb_norm_super(r, p, 0);
		return;
	}
#endif /* EB_SUPER */

#if defined(EB_ORDIN) || defined(EB_KBLTZ)
	eb_norm_ordin(r, p, 0);
#endif /* EB_ORDIN || EB_KBLTZ */

#endif /* EB_ADD == PROJC */
}

void eb_norm_sim(eb_t *r, eb_t *t, int n) {
	int i;
	fb_t a[n];

	if (n == 1) {
		eb_norm(r[0], t[0]);
		return;
	}

	for (i = 0; i < n; i++) {
		fb_null(a[i]);
	}

	TRY {
		for (i = 0; i < n; i++) {
			fb_new(a[i]);
			fb_copy(a[i], t[i]->z);
		}

		fb_inv_sim(a, a, n);

		for (i = 0; i < n; i++) {
			fb_copy(t[i]->z, a[i]);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < n; i++) {
			fb_free(a[i]);
		}
	}

#if defined(EB_SUPER)
	if (eb_curve_is_super()) {
		for (i = 0; i < n; i++) {
			eb_norm_super(r[i], t[i], 1);
		}
	}
#endif
#if defined(EB_ORDIN) || defined(EB_KBLTZ)
	if (!eb_curve_is_super()) {
		for (i = 0; i < n; i++) {
			eb_norm_ordin(r[i], t[i], 1);
		}
	}
#endif
}
