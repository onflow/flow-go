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
 * Implementation of the ternary field cubing.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_bn_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FB_CUB == BASIC || !defined(STRIP)

void ft_cub_basic(ft_t c, ft_t a) {
	ft_mul(c, a, a);
	ft_mul(c, c, a);
}

#endif

#if FB_CUB == TABLE || !defined(STRIP)

void ft_cub_table(ft_t c, ft_t a) {
	dv_t t, t2;

	dv_null(t);

	TRY {
		/* We need a temporary variable so that c can be a or b. */
		dv_new(t);
		ft_cubl_low(t, a);
		ft_rdc_cub(c, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

#endif

#if FB_CUB == INTEG || !defined(STRIP)

void ft_cub_integ(ft_t c, ft_t a) {
	ft_cubm_low(c, a);
}

#endif
