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
 * Implementation of the ternary field modulus manipulation.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_dv.h"
#include "relic_ft.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Current configured ternary field identifier.
 */
static int param_id;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int ft_param_get(void) {
	return param_id;
}

void ft_param_set(int param) {
	switch (param) {
		case TRINO_97:
			ft_poly_set_trino(12, -1);
			break;
		case PENTA_509:
			ft_poly_set_penta(-318, -191, 127, 1);
			break;
		default:
			THROW(ERR_INVALID);
			break;
	}
	param = param_id;
}

void ft_param_set_any(void) {
#if FT_POLYN == 97
	ft_param_set(TRINO_97);
#elif FT_POLYN == 509
	ft_param_set(PENTA_509);
#else
	THROW(ERR_NO_FIELD);
#endif
}

void ft_param_print(void) {
	int fa, fb, fc, fd;

	ft_poly_get_rdc(&fa, &fb, &fc, &fd);

	if (fc == 0) {
		util_print_banner("Irreducible trinomial:", 0);
		util_print("   z^%d", FT_TRITS);
		if (fa > 0) {
			util_print(" + z^%d", fa);
		} else {
			util_print(" - z^%d", fa);
		}
		if (fb > 0) {
			util_print(" + 1\n");
		} else {
			util_print(" - 1\n");
		}
	} else {
		util_print_banner("Irreducible pentanomial:", 0);
		util_print("   z^%d", FT_TRITS);
		if (fa > 0) {
			util_print(" + z^%d", fa);
		} else {
			util_print(" - z^%d", -fa);
		}
		if (fb > 0) {
			util_print(" + z^%d", fb);
		} else {
			util_print(" - z^%d", -fb);
		}
		if (fc > 0) {
			util_print(" + z^%d", fc);
		} else {
			util_print(" - z^%d", -fc);
		}
		if (fd > 0) {
			util_print(" + 1\n");
		} else {
			util_print(" - 1\n");
		}
	}
}
