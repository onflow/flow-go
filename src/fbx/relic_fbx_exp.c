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
 * Implementation of exponentiation in extensions defined over binary fields.
 *
 * @version $Id$
 * @ingroup fbx
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_util.h"
#include "relic_pb.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb4_exp(fb4_t c, fb4_t a, bn_t b) {
	fb4_t t;

	fb4_null(t);

	TRY {
		fb4_new(t);

		fb4_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fb4_sqr(t, t);
			if (bn_test_bit(b, i)) {
				fb4_mul(t, t, a);
			}
		}
		fb4_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb4_free(t);
	}
}

void fb6_exp(fb6_t c, fb6_t a, bn_t b) {
	fb6_t t;

	fb6_null(t);

	TRY {
		fb6_new(t);

		fb6_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fb6_sqr(t, t);
			if (bn_test_bit(b, i)) {
				fb6_mul(t, t, a);
			}
		}
		fb6_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb6_free(t);
	}
}

void fb12_exp(fb12_t c, fb12_t a, bn_t b) {
	fb12_t t;

	fb12_null(t);

	TRY {
		fb12_new(t);

		fb12_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fb12_sqr(t, t);
			if (bn_test_bit(b, i)) {
				fb12_mul(t, t, a);
			}
		}
		fb12_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb12_free(t);
	}
}
