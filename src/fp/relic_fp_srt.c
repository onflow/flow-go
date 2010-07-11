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
 * Implementation of the square root functions.
 *
 * @version $Id$
 * @ingroup bn
 */

#include <string.h>

#include "relic_core.h"
#include "relic_fp.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int fp_srt(fp_t c, fp_t a) {
	bn_t e;
	fp_t t1;
	fp_t t2;
	int r = 0;

	bn_null(e);
	fp_null(t1);
	fp_null(t2);

	if (fp_prime_get_mod8() != 3 && fp_prime_get_mod8() != 7) {
		THROW(ERR_INVALID);
		return 0;
	}

	TRY {
		bn_new(e);
		fp_new(t1);
		fp_new(t2);

		e->used = FP_DIGS;
		dv_copy(e->dp, fp_prime_get(), FP_DIGS);
		bn_add_dig(e, e, 1);
		bn_rsh(e, e, 2);

		fp_exp(t1, a, e);

		fp_sqr(t2, t1);
		r = (fp_cmp(t2, a) == CMP_EQ);
		fp_copy(c, t1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(e);
		fp_free(t1);
		fp_free(t2);
	}
	return r;
}
