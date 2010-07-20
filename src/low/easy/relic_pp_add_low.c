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
 * Implementation of the low-level quadratic extension field multiplication
 * functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include "relic_fp.h"
#include "relic_pp.h"
#include "relic_core.h"
#include "relic_error.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp2_addn_low(fp2_t c, fp2_t a, fp2_t b) {
  fp_add(c[0], a[0], b[0]);
  fp_add(c[1], a[1], b[1]);
}

void fp2_subn_low(fp2_t c, fp2_t a, fp2_t b) {
  fp_sub(c[0], a[0], b[0]);
  fp_sub(c[1], a[1], b[1]);
}

void fp2_dbln_low(fp2_t c, fp2_t a) {
  /* 2 * (a0 + a1 * u) = 2 * a0 + 2 * a1 * u. */
  fp_dbl(c[0], a[0]);
  fp_dbl(c[1], a[1]);
}
