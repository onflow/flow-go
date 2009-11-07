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
 * @defgroup ep Prime elliptic curves.
 */

/**
 * @file
 *
 * Interface of the prime elliptic curves functions.
 *
 * @version $Id$
 * @ingroup ep
 */

#ifndef RELIC_EP2_H
#define RELIC_EP2_H

#include "relic_fp2.h"
#include "relic_bn.h"
#include "relic_types.h"

/**
 * Represents an ellyptic curve point over a quadratic extension prime field.
 */
typedef struct {
	/** The first coordinate. */
	fp2_t x;
	/** The second coordinate. */
	fp2_t y;
	/** The third coordinate (projective representation). */
	fp2_t z;
	/** Flag to indicate that this point is normalized. */
	int norm;
} ep2_st;

/**
 * Pointer to an elliptic curve point.
 */
typedef ep2_st *ep2_t;

/**
 * Returns the a coefficient of the currently configured prime elliptic curve.
 *
 * @return the a coefficient of the elliptic curve.
 */
dig_t *ep2_curve_get_a(void);

/**
 * Returns the b coefficient of the currently configured prime elliptic curve.
 *
 * @return the b coefficient of the elliptic curve.
 */
dig_t *ep2_curve_get_b(void);

/**
 * Returns a optimization identifier based on the coefficient a of the curve.
 *
 * @return the optimization identifier.
 */
int ep2_curve_opt_a(void);

/**
 * Tests if the configured prime elliptic curve is supersingular.
 *
 * @return 1 if the prime elliptic curve is supersingular, 0 otherwise.
 */
int ep2_curve_is_super(void);

/**
 * Returns the generator of the group of points in the prime elliptic curve.
 *
 * @param[out] g			- the point to store the generator.
 */
ep2_t ep2_curve_get_gen(void);

/**
 * Returns the order of the group of points in the prime elliptic curve.
 *
 * @param[out] r			- the multiple precision integer to store the order.
 */
bn_t ep2_curve_get_ord(void);

/**
 * Configures a new ordinary prime elliptic curve by its coefficients.
 *
 * @param[in] a			- the coefficient a of the curve.
 * @param[in] b			- the coefficient b of the curve.
 */
void ep2_curve_set_ordin(fp2_t a, fp2_t b);

/**
 * Configures a new generator for the group of points in the prime elliptic
 * curve.
 *
 * @param[in] g			- the new generator.
 */
void ep2_curve_set_gen(ep2_t g);

/**
 * Changes the order of the group of points.
 *
 * @param[in] r			- the new order.
 */
void ep2_curve_set_ord(bn_t r);

/**
 * Configures a new prime elliptic curve by its parameter identifier.
 *
 * @param				- the parameters identifier.
 */
void ep2_param_set(int param);

/**
 * Calls a function to allocate a point on a prime elliptic curve.
 *
 * @param[out] A			- the new point.
 */
#if ALLOC == DYNAMIC
#define ep2_new(A)															\
	A = (ep2_t)malloc(sizeof(ep2_st));										\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	fp2_new((A)->x);														\
	fp2_new((A)->y);														\
	fp2_new((A)->z);														\

#elif ALLOC == STATIC
#define ep2_new(A)															\
	A = (ep2_st *)alloca(sizeof(ep2_st));									\
	fp2_new((A)->x);														\
	fp2_new((A)->y);														\
	fp2_new((A)->z);														\

#elif ALLOC == STACK
#define ep2_new(A)															\
	A = (ep2_t)alloca(sizeof(ep2_st));										\
	fp2_new((A)->x);														\
	fp2_new((A)->y);														\
	fp2_new((A)->z);														\

#endif

/**
 * Calls a function to clean and free a point on a prime elliptic curve.
 *
 * @param[out] A			- the point to free.
 */
#if ALLOC == DYNAMIC
#define ep2_free(A)															\
	if (A != NULL) {														\
		fp2_free((A)->x);													\
		fp2_free((A)->y);													\
		fp2_free((A)->z);													\
		free(A);															\
		A = NULL;															\
	}

#elif ALLOC == STATIC
#define ep2_free(A)															\
	fp2_free((A)->x);														\
	fp2_free((A)->y);														\
	fp2_free((A)->z);														\
	A = NULL;																\

#elif ALLOC == STACK
#define ep2_free(A)															\
	A = NULL;																\

#endif

/**
 * Tests if a point on a prime elliptic curve is at the infinity.
 *
 * @param[in] p				- the point to test.
 * @return 1 if the point is at infinity, 0 otherise.
 */
int ep2_is_infinity(ep2_t p);

/**
 * Assigns a prime elliptic curve point to a point at the infinity.
 *
 * @param[out] p			- the point to assign.
 */
void ep2_set_infinity(ep2_t p);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] q			- the result.
 * @param[in] p				- the prime elliptic curve point to copy.
 */
void ep2_copy(ep2_t r, ep2_t p);

/**
 * Compares two prime elliptic curve points.
 *
 * @param[in] p				- the first prime elliptic curve point.
 * @param[in] q				- the second prime elliptic curve point.
 * @return CMP_EQ if p == q and CMP_NE if p != q.
 */
int ep2_cmp(ep2_t p, ep2_t q);

/**
 * Assigns a random value to a prime elliptic curve point.
 *
 * @param[out] p			- the prime elliptic curve point to assign.
 */
void ep2_rand(ep2_t p);

/**
 * Prints a prime elliptic curve point.
 *
 * @param[in] p				- the prime elliptic curve point to print.
 */
void ep2_print(ep2_t p);

/**
 * Negates a prime elliptic curve point.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the point to negate.
 */
#if EP_ADD == BASIC
#define ep2_neg(R, P)		ep2_neg_basic(R, P)
#elif EP_ADD == PROJC
#define ep2_neg(R, P)		ep2_neg_projc(R, P)
#endif

/**
 * Negates a prime elliptic curve point represented by Affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to negate.
 */
void ep2_neg_basic(ep2_t r, ep2_t p);

/**
 * Negates a prime elliptic curve point represented by López-Dahab Projective
 * coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to negate.
 */
void ep2_neg_projc(ep2_t r, ep2_t p);

/**
 * Adds two prime elliptic curve points.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first point to add.
 * @param[in] Q				- the second point to add.
 */
#if EP_ADD == BASIC
#define ep2_add(R, P, Q)		ep2_add_basic(R, P, Q);
#elif EP_ADD == PROJC
#define ep2_add(R, P, Q)		ep2_add_projc(R, P, Q);
#endif

/**
 * Adds two prime elliptic curve points represented in Affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to add.
 * @param[in] q				- the second point to add.
 */
void ep2_add_basic(ep2_t r, ep2_t p, ep2_t q);

/**
 * Adds two prime elliptic curve points returning the slope.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first point to add.
 * @param[in] Q				- the second point to add.
 */
#if EP_ADD == BASIC
#define ep2_add_slope(R, S, P, Q)	ep2_add_basic_slope(R, S, P, Q);
#elif EP_ADD == PROJC
#define ep2_add_slope(R, S, P, Q)	ep2_add_projc_slope(R, S, P, Q);
#endif

/**
 * Adds two prime elliptic curve points represented in Affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to add.
 * @param[in] q				- the second point to add.
 */
void ep2_add_basic_slope(ep2_t r, fp2_t s, ep2_t p, ep2_t q);

void ep2_add_projc_slope(ep2_t r, fp2_t s, ep2_t p, ep2_t q);

/**
 * Doubles a prime elliptic curve point.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the point to double.
 */
#if EP_ADD == BASIC
#define ep2_dbl(R, P)		ep2_dbl_basic(R, P);
#elif EP_ADD == PROJC
#define ep2_dbl(R, P)		ep2_dbl_projc(R, P);
#endif

/**
 * Doubles a prime elliptic curve point represented in affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to double.
 */
void ep2_dbl_basic(ep2_t r, ep2_t p);

void ep2_dbl_projc(ep2_t r, ep2_t p);

/**
 * Doubles a prime elliptic curve point returning the slope.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the point to double.
 */
#if EP_ADD == BASIC
#define ep2_dbl_slope(R, S, E, P)	ep2_dbl_basic_slope(R, S, E, P);
#elif EP_ADD == PROJC
#define ep2_dbl_slope(R, S, P)	ep2_dbl_projc_slope(R, S, P);
#endif

/**
 * Doubles a prime elliptic curve point represented in affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to double.
 */
void ep2_dbl_basic_slope(ep2_t r, fp2_t s, fp2_t e, ep2_t p);

/**
 * Multiplies a prime elliptic point by an integer using the Binary method.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to multiply.
 * @param[in] k				- the integer.
 */
void ep2_mul_basic(ep2_t r, ep2_t p, bn_t k);

/**
 * Converts a point to affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to convert.
 */
void ep2_norm(ep2_t r, ep2_t p);

void ep2_mul(ep2_t r, ep2_t p, bn_t k);

void ep2_curve_set_twist(int twist);

#endif /* !RELIC_EP2_H */
