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
 * Implementation of the prime elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup ep
 */

#include <string.h>

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#define RATE_P256_A	"0"
#define RATE_P256_B	"16"
#define RATE_P256_P	"2523648240000001BA344D80000000086121000000000013A700000000000013"
#define RATE_P256_X "1"
#define RATE_P256_Y "C7424FC261B627189A14E3433B4713E9C2413FCF89B8E2B178FB6322EFB2AB3"
#define RATE_P256_R "2523648240000001BA344D8000000007FF9F800000000010A10000000000000D"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ep_param_set(int param) {
	fp_t a, b;
	bn_t p, r;
	ep_t g;

	bn_new(p);
	bn_new(r);
	fp_new(a);
	fp_new(b);
	ep_new(g);

	switch (param) {
		case RATE_P256:
			bn_read_str(p, RATE_P256_P, strlen(RATE_P256_P), 16);
			fp_prime_set_dense(p);
			fp_read(a, RATE_P256_A, strlen(RATE_P256_A), 16);
			fp_read(b, RATE_P256_B, strlen(RATE_P256_B), 16);
			bn_read_str(r, RATE_P256_R, strlen(RATE_P256_R), 16);
			fp_read(g->x, RATE_P256_X, strlen(RATE_P256_X), 16);
			fp_read(g->y, RATE_P256_Y, strlen(RATE_P256_Y), 16);
			break;
		default:
			THROW(ERR_INVALID);
			break;
	}
	fp_zero(g->z);
	fp_add_dig(g->z, g->z, 1);
	g->norm = 1;
	ep_curve_set_ordin(a, b);
	ep_curve_set_gen(g);
	ep_curve_set_ord(r);

	bn_free(p);
	bn_free(r);
	ep_free(g);
	fp_free(a);
	fp_free(b);
}
