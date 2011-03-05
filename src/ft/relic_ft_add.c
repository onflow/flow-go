/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Implementation of ternary field addition and subtraction functions.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_add(ft_t c, ft_t a, ft_t b) {
	ft_addn_low(c, a, b);
}

void ft_add_dig(ft_t c, ft_t a, dig_t b) {
	ft_add1_low(c, a, b);
}

void ft_sub(ft_t c, ft_t a, ft_t b) {
	ft_subn_low(c, a, b);
}

void ft_sub_dig(ft_t c, ft_t a, dig_t b) {
	ft_sub1_low(c, a, b);
}
