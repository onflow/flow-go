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
 * Implementation of the Sakai-Ohgishi-Kasahara Identity-Based Non-Interactive
 * Authenticated Key Agreement scheme.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>
#include<string.h>
#include<math.h>
#include<stdlib.h>
#include<stdint.h>

#include "relic.h"
#include "relic_test.h"
#include "relic_bench.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void cp_sokaka_gen(bn_t master) {
	bn_t n;

	bn_null(n);

	n = eb_curve_get_ord();

	do {
		bn_rand(master, BN_POS, bn_bits(n));
		bn_mod(master, master, n);
	} while (bn_is_zero(master));
}

void cp_sokaka_gen_pub(eb_t p, char *id, int len) {
	eb_map(p, (unsigned char *)id, len);
}

void cp_sokaka_gen_prv(eb_t s, char *id, int len, bn_t master) {
	eb_map(s, (unsigned char *)id, len);
	eb_mul(s, s, master);
}

void cp_sokaka_key(fb4_t key, eb_t p, eb_t s) {
	pb_map(key, p, s);
}
