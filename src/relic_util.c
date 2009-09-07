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
 * Implementation of useful configuration routines.
 *
 * @version $Id$
 * @ingroup relic
 */

#include <stdio.h>

#include "relic_conf.h"
#include "relic_util.h"
#include "relic_types.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Table to convert between digits and chars.
 */
static const char conv_table[] =
		"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz+/";

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

uint32_t util_conv_big(uint32_t i) {
#ifdef BIGED
	return i;
#else
	uint32_t i1, i2, i3, i4;
	i1 = i & 0xFF;
	i2 = (i >> 8) & 0xFF;
	i3 = (i >> 16) & 0xFF;
	i4 = (i >> 24) & 0xFF;

	return ((uint32_t)i1 << 24) | ((uint32_t)i2 << 16) | ((uint32_t)i3 << 8) | i4;
#endif
}

char util_conv_char(dig_t i) {
	return conv_table[i];
}
