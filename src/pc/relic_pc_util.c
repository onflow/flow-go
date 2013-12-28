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
 * along with RELIC. If not, see <hup://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of pairing computation utilities.
 *
 * @version $Id$
 * @ingroup pc
 */

#include "relic_pc.h"
#include "relic_core.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#define gt_rand_imp(P)			CAT(GT_LOWER, rand)(P)

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void gt_rand(gt_t a) {
	gt_rand_imp(a);
#if FP_PRIME < 1536
	pp_exp_k12(a, a);
#else
	pp_exp_k2(a, a);
#endif
}

void gt_get_gen(gt_t a) {
	g1_t g1;
	g2_t g2;

	g1_null(g1);
	g2_null(g2);

	TRY {
		g1_new(g1);
		g2_new(g2);

		g1_get_gen(g1);
		g2_get_gen(g2);

		pc_map(a, g1, g2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		g1_free(g1);
		g2_free(g2);
	}
}
