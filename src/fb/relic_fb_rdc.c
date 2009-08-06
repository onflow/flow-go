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
 * Implementation of the binary field modular reduction.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_bn_low.h"
#include "relic_dv.h"
#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FB_RDC == BASIC || !defined(STRIP)

void fb_rdc_basic(fb_t c, dv_t a) {
	int j, k;
	dig_t *tmpa, *rdc;

	tmpa = a + FB_DIGS;

	/* First reduce the high part. */
	for (int i = fb_bits(tmpa); i >= 0; i--) {
		if (fb_test_bit(tmpa, i)) {
			SPLIT(k, j, i - FB_BITS, FB_DIG_LOG);
			rdc = fb_poly_get_rdc(k);
			fb_addd_low(tmpa + j, tmpa + j, rdc, FB_DIGS + 1);
		}
	}
	for (int i = fb_bits(a); i >= FB_BITS; i--) {
		if (fb_test_bit(a, i)) {
			SPLIT(k, j, i - FB_BITS, FB_DIG_LOG);
			rdc = fb_poly_get_rdc(k);
			fb_addd_low(a + j, a + j, rdc, FB_DIGS + 1);
		}
	}

	fb_copy(c, a);
}

#endif

#if FB_RDC == QUICK || !defined(STRIP)

void fb_rdc_quick(fb_t c, dv_t a) {
	fb_rdcn_low(c, a);
}

#endif
