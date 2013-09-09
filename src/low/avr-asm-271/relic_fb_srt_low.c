/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Implementation of the low-level binary field square root.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_util.h"

void fb_srtp_low(dig_t *c, dig_t *t, dig_t *a);

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb_srtn_low(dig_t *c, const dig_t *a) {
	dig_t t[FB_DIGS] = { 0 };

	fb_srtp_low(c, t, a);
}
