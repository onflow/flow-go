/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
 * Implementation of the quartic extension binary field arithmetic module.
 *
 * The implementations of fb4_mul() and fb4_mul_dxs() are based on:
 *
 * Beuchat et al., A comparison between hardware accelerators for the modified
 * Tate pairing over F_2^m and F_3^m, 2008.
 *
 * The implementation of fb4_mul_dxd() is based on:
 *
 * Shirase et al., Efficient computation of Eta pairing over binary field with
 * Vandermonde matrix, 2008.
 *
 * and optimal 4-way Toom-Cook from:
 *
 * Bodrato, Towards optimal Toom-Cook multiplication for univariate and
 * multivariate polynomials in characteristic 2 and 0, 2007.
 *
 * @version $Id$
 * @ingroup pb
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_util.h"
#include "relic_pb.h"
#include "relic_fb_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

