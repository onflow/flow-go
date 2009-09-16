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
 * @defgroup rate R-ate bilinear pairing.
 */

/**
 * @file
 *
 * Interface of the r-ate bilinear pairing functions.
 *
 * @version $Id$
 * @ingroup pp
 */

#ifndef RELIC_PP_H
#define RELIC_PP_H

#include "relic_fp.h"
#include "relic_fp2.h"
#include "relic_fp6.h"
#include "relic_fp12.h"
#include "relic_ep.h"
#include "relic_ep2.h"
#include "relic_types.h"

/**
 * Compute the R-Ate pairing.
 * 
 * @param[out] r			- the result.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] p				- the second point of the pairing, in G_1.
 * @param[in] x				- the BN parameter used to generate the curve.
 * @param[in] f				- constant used in Frobenius, Z^p.
 */
void pp_pair_rate(fp12_t res, ep2_t q, ep_t p, bn_t x, fp12_t f);

#endif /* !RELIC_PP_H */
