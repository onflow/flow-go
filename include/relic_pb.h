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
 * @defgroup pb Pairings over binary elliptic curves.
 */

/**
 * @file
 *
 * Interface of the module for computing bilinear pairings over binary elliptic
 * curves.
 *
 * @version $Id$
 * @ingroup pb
 */

#ifndef RELIC_PB_H
#define RELIC_PB_H

#include "relic_fb.h"
#include "relic_fbx.h"
#include "relic_eb.h"
#include "relic_types.h"

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Computes the pairing of two binary elliptic curve points. Computes
 * R = e(P, Q).
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first elliptic curve point.
 * @param[in] Q				- the second elliptic curve point.
 */
#define pb_map_gens1(R, P, Q)		pb_map_etats(R, P, Q)
#if PB_MAP == ETATN
#undef pb_map_gens1
#define pb_map_gens1(R, P, Q)		pb_map_etatn(R, P, Q)
#endif

/**
 * Computes the pairing of two binary hyperelliptic divisors. Computes
 * R = e(P, Q).
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first divisor.
 * @param[in] Q				- the second divisor.
 */
#if PB_MAP == ETAT2
#define pb_map_gens2(R, P, Q)		pb_map_etat2(R, P, Q)
#elif PB_MAP == OETA2
#define pb_map_gens2(R, P, Q)		pb_map_oeta2(R, P, Q)
#endif

/**
 * Computes the pairing of two points. Computes R = e(P, Q).
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first elliptic curve point.
 * @param[in] Q				- the second elliptic curve point.
 */
#if PB_MAP == ETATS || PB_MAP == ETATN
#define pb_map(R, P, Q)				pb_map_gens1(R, P, Q)
#elif PB_MAP == ETAT2 || PB_MAP == OETA2
#define pb_map(R, P, Q)				pb_map_gens2(R, P, Q)
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Initializes the module for computing pairings over binary fields.
 */
void pb_map_init();

/**
 * Finalizes the module for computing pairings over binary fields.
 */
void pb_map_clean();

/**
 * Returns the table for computing the final exponentiation in the selected
 * pairing.
 *
 * @return The precomputed table.
 */
const fb_t *pb_map_get_tab();

/**
 * Returns the table for computing the repeated squarings needed for parallel
 * execution.
 *
 * @return The precomputed table.
 */
const fb_t *pb_map_get_sqr();

/**
 * Returns the table for computing the repeated square-roots needed for parallel
 * execution.
 *
 * @return The precomputed table.
 */
const fb_t *pb_map_get_srt();

/**
 * Computes the etat pairing of two binary elliptic curve points without using
 * square roots.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first elliptic curve point.
 * @param[in] q				- the second elliptic curve point.
 */
void pb_map_etats(fb4_t r, const eb_t p, const eb_t q);

/**
 * Computes the etat pairing of two binary elliptic curve points using
 * square roots.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first elliptic curve point.
 * @param[in] q				- the second elliptic curve point.
 */
void pb_map_etatn(fb4_t r, const eb_t p, const eb_t q);

#endif /* !RELIC_PB_H */
