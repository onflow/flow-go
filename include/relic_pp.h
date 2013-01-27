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
 * @defgroup pp Pairings over prime elliptic curves.
 */

/**
 * @file
 *
 * Interface of the module for computing bilinear pairings over prime elliptic
 * curves.
 *
 * @version $Id$
 * @ingroup pp
 */

#ifndef RELIC_PP_H
#define RELIC_PP_H

#include "relic_fpx.h"
#include "relic_epx.h"
#include "relic_types.h"

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Adds two prime elliptic curve points and evaluates the corresponding line
 * function at another elliptic curve point using projective coordinates.
 *
 * @param[out] L			- the result of the evaluation.
 * @param[in, out] R		- the resulting point and first point to add.
 * @param[in] Q				- the second point to add.
 * @param[in] P				- the affine point to evaluate the line function.
 */
#if PP_EXT == BASIC
#define pp_add_k12_projc(L, R, Q, P)	pp_add_k12_projc_basic(L, R, Q, P)
#elif PP_EXT == LAZYR
#define pp_add_k12_projc(L, R, Q, P)	pp_add_k12_projc_lazyr(L, R, Q, P)
#endif


/**
 * Doubles a prime elliptic curve point and evaluates the corresponding line
 * function at another elliptic curve point.
 *
 * @param[out] L			- the result of the evaluation.
 * @param[out] R			- the resulting point.
 * @param[in] Q				- the point to double.
 * @param[in] P				- the affine point to evaluate the line function.
 */
#if EP_ADD == BASIC
#define pp_dbl_k12(L, R, Q, P)			pp_dbl_k12_basic(L, R, Q, P)
#elif EP_ADD == PROJC
#define pp_dbl_k12(L, R, Q, P)			pp_dbl_k12_projc(L, R, Q, P)
#endif

/**
 * Doubles a prime elliptic curve point and evaluates the corresponding line
 * function at another elliptic curve point using projective coordinates.
 *
 * @param[out] L			- the result of the evaluation.
 * @param[in, out] R		- the resulting point.
 * @param[in] Q				- the point to double.
 * @param[in] P				- the affine point to evaluate the line function.
 */
#if PP_EXT == BASIC
#define pp_dbl_k12_projc(L, R, Q, P)	pp_dbl_k12_projc_basic(L, R, Q, P)
#elif PP_EXT == LAZYR
#define pp_dbl_k12_projc(L, R, Q, P)	pp_dbl_k12_projc_lazyr(L, R, Q, P)
#endif

/**
 * Computes a pairing of two binary elliptic curve points. Computes e(P, Q).
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first elliptic curve point.
 * @param[in] Q				- the second elliptic curve point.
 */
#if PP_MAP == TATEP
#define pp_map(R, P, Q)				pp_map_tatep(R, P, Q)
#elif PP_MAP == WEILP
#define pp_map(R, P, Q)				pp_map_weilp(R, P, Q)
#elif PP_MAP == OATEP
#define pp_map(R, P, Q)				pp_map_oatep(R, P, Q)
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Initializes the pairing over prime fields.
 */
void pp_map_init(void);

/**
 * Finalizes the pairing over prime fields.
 */
void pp_map_clean(void);

/**
 * Adds two prime elliptic curve points and evaluates the corresponding line
 * function at another elliptic curve point using affine coordinates.
 *
 * @param[out] l			- the result of the evaluation.
 * @param[in, out] r		- the resulting point and first point to add.
 * @param[in] q				- the second point to add.
 * @param[in] p				- the affine point to evaluate the line function.
 */
void pp_add_k12_basic(fp12_t l, ep2_t r, ep2_t q, ep_t p);

/**
 * Adds two prime elliptic curve points and evaluates the corresponding line
 * function at another elliptic curve point using projective coordinates.
 *
 * @param[out] l			- the result of the evaluation.
 * @param[in, out] r		- the resulting point and first point to add.
 * @param[in] q				- the second point to add.
 * @param[in] p				- the affine point to evaluate the line function.
 */
void pp_add_k12_projc_basic(fp12_t l, ep2_t r, ep2_t q, ep_t p);

/**
 * Adds two prime elliptic curve points and evaluates the corresponding line
 * function at another elliptic curve point using projective coordinates and
 * lazy reduction.
 *
 * @param[out] l			- the result of the evaluation.
 * @param[in, out] r		- the resulting point and first point to add.
 * @param[in] q				- the second point to add.
 * @param[in] p				- the affine point to evaluate the line function.
 */
void pp_add_k12_projc_lazyr(fp12_t l, ep2_t r, ep2_t q, ep_t p);

/**
 * Adds two prime elliptic curve points and evaluates the corresponding line
 * function at another elliptic curve point using projective coordinates.
 *
 * @param[out] l			- the result of the evaluation.
 * @param[in, out] r		- the resulting point and first point to add.
 * @param[in] p				- the second point to add.
 * @param[in] q				- the affine point to evaluate the line function.
 */
void pp_add_lit_k12(fp12_t l, ep_t r, ep_t p, ep2_t q);

/**
 * Doubles a prime elliptic curve point and evaluates the corresponding line
 * function at another elliptic curve point using affine coordinates.
 *
 * @param[out] l			- the result of the evaluation.
 * @param[in, out] r		- the resulting point.
 * @param[in] q				- the point to double.
 * @param[in] p				- the affine point to evaluate the line function.
 */
void pp_dbl_k12_basic(fp12_t l, ep2_t r, ep2_t q, ep_t p);

/**
 * Doubles a prime elliptic curve point and evaluates the corresponding line
 * function at another elliptic curve point using projective coordinates.
 *
 * @param[out] l			- the result of the evaluation.
 * @param[in, out] r		- the resulting point.
 * @param[in] q				- the point to double.
 * @param[in] p				- the affine point to evaluate the line function.
 */
void pp_dbl_k12_projc_basic(fp12_t l, ep2_t r, ep2_t q, ep_t p);

/**
 * Doubles a prime elliptic curve point and evaluates the corresponding line
 * function at another elliptic curve point using projective coordinates and
 * lazy reduction.
 *
 * @param[out] l			- the result of the evaluation.
 * @param[in, out] r		- the resulting point.
 * @param[in] q				- the point to double.
 * @param[in] p				- the affine point to evaluate the line function.
 */
void pp_dbl_k12_projc_lazyr(fp12_t l, ep2_t r, ep2_t q, ep_t p);

/**
 * Doubles a prime elliptic curve point and evaluates the corresponding line
 * function at another elliptic curve point using affine coordinates.
 *
 * @param[out] l			- the result of the evaluation.
 * @param[in, out] r		- the resulting point.
 * @param[in] p				- the point to double.
 * @param[in] q				- the affine point to evaluate the line function.
 */
void pp_dbl_lit_k12(fp12_t l, ep_t r, ep_t p, ep2_t q);

/**
 * Computes the final exponentiation for the pairing defined over curves of
 * embedding degree 12. Computes c = a^(p^12 - 1)/r.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the extension field element to exponentiate.
 */
void pp_exp_k12(fp12_t c, fp12_t a);

/**
 * Normalizes the accumulator point used inside pairing computation.
 *
 * @param[out] r			- the resulting point.
 * @param[in] p				- the point to normalize.
 */
void pp_norm(ep2_t c, ep2_t a);

/**
 * Computes the Tate pairing of two points in a parameterized elliptic curve.
 *
 * @param[out] r			- the result.
 * @param[in] q				- the first elliptic curve point.
 * @param[in] p				- the second elliptic curve point.
 */
void pp_map_tatep(fp12_t r, ep_t p, ep2_t q);

/**
 * Computes the Weil pairing of two points in a parameterized elliptic
 * curve.
 *
 * @param[out] r			- the result.
 * @param[in] q				- the first elliptic curve point.
 * @param[in] p				- the second elliptic curve point.
 */
void pp_map_weilp(fp12_t r, ep_t p, ep2_t q);

/**
 * Computes the optimal ate pairing of two points in a parameterized elliptic
 * curve.
 *
 * @param[out] r			- the result.
 * @param[in] q				- the first elliptic curve point.
 * @param[in] p				- the second elliptic curve point.
 */
void pp_map_oatep(fp12_t r, ep_t p, ep2_t q);

#endif /* !RELIC_PP_H */
