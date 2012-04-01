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
 * Implementation of frobenius action in extensions defined over prime fields.
 *
 * @version $Id$
 * @ingroup fpx
 */

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_pp_low.h"
#include "relic_pp.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp2_frb(fp2_t c, fp2_t a, int i) {
	switch (i % 2) {
		case 0:
			fp2_copy(c, a);
			break;
		case 1:
			/* (a_0 + a_1 * u)^p = a_0 - a_1 * u. */
			fp_copy(c[0], a[0]);
			fp_neg(c[1], a[1]);
			break;
	}
}

void fp6_frb(fp6_t c, fp6_t a, int i) {
	if (i == 1) {
		fp2_frb(c[0], a[0], 1);
		fp2_frb(c[1], a[1], 1);
		fp2_frb(c[2], a[2], 1);
		fp2_mul_frb(c[1], c[1], 1, 2);
		fp2_mul_frb(c[2], c[2], 1, 4);
	} else {
		fp2_copy(c[0], a[0]);
		fp2_mul_frb(c[1], a[1], 2, 2);
		fp2_mul_frb(c[2], a[2], 2, 1);
		fp2_neg(c[2], c[2]);
	}
}

void fp12_frb(fp12_t c, fp12_t a, int i) {
	switch (i) {
		case 2:
			fp2_copy(c[0][0], a[0][0]);
			fp2_mul_frb(c[0][2], a[0][2], 2, 1);
			fp2_mul_frb(c[0][1], a[0][1], 2, 2);
			fp2_neg(c[0][2], c[0][2]);
			fp2_mul_frb(c[1][0], a[1][0], 2, 1);
			fp2_mul_frb(c[1][2], a[1][2], 2, 2);
			fp2_mul_frb(c[1][1], a[1][1], 2, 3);
			fp2_neg(c[1][2], c[1][2]);
			break;
		case 1:
			fp2_frb(c[0][0], a[0][0], 1);
			fp2_frb(c[1][0], a[1][0], 1);
			fp2_frb(c[0][1], a[0][1], 1);
			fp2_frb(c[1][1], a[1][1], 1);
			fp2_frb(c[0][2], a[0][2], 1);
			fp2_frb(c[1][2], a[1][2], 1);
			fp2_mul_frb(c[1][0], c[1][0], 1, 1);
			fp2_mul_frb(c[0][1], c[0][1], 1, 2);
			fp2_mul_frb(c[1][1], c[1][1], 1, 3);
			fp2_mul_frb(c[0][2], c[0][2], 1, 4);
			fp2_mul_frb(c[1][2], c[1][2], 1, 5);
			break;
		case 3:
			fp2_frb(c[0][0], a[0][0], 1);
			fp2_frb(c[1][0], a[1][0], 1);
			fp2_frb(c[0][1], a[0][1], 1);
			fp2_frb(c[1][1], a[1][1], 1);
			fp2_frb(c[0][2], a[0][2], 1);
			fp2_frb(c[1][2], a[1][2], 1);
			fp2_mul_frb(c[0][1], c[0][1], 3, 2);
			fp2_mul_frb(c[0][2], c[0][2], 3, 4);
			fp2_neg(c[0][2], c[0][2]);
			fp2_mul_frb(c[1][0], c[1][0], 3, 1);
			fp2_mul_frb(c[1][1], c[1][1], 3, 3);
			fp2_mul_frb(c[1][2], c[1][2], 3, 5);
			fp2_neg(c[1][2], c[1][2]);
			break;
	}
}
