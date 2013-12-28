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
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of frobenius action in binary fields extensions.
 *
 * @version $Id$
 * @ingroup fbx
 */

#include "relic_core.h"
#include "relic_fbx.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb4_frb(fb4_t c, fb4_t a) {
	int alpha;

	if (FB_BITS % 4 == 3) {
		alpha = 0;
	} else {
		alpha = 1;
	}

	if (alpha == 1) {
		fb_add(c[0], a[0], a[1]);
		fb_add(c[0], c[0], a[3]);
	} else {
		fb_add(c[0], a[0], a[1]);
		fb_add(c[0], c[0], a[2]);
	}
	fb_add(c[1], a[1], a[2]);
	if (alpha == 0) {
		fb_add(c[1], c[1], a[3]);
	}
	fb_add(c[2], a[2], a[3]);
	fb_copy(c[3], a[3]);
}
