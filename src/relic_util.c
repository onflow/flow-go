/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of useful configuration routines.
 *
 * @version $Id: relic_util.c 13 2009-04-16 02:24:55Z dfaranha $
 * @ingroup relic
 */

#include <stdio.h>

#include "relic_conf.h"
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

char util_conv(dig_t i) {
	return conv_table[i];
}
