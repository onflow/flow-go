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

#ifndef RELIC_EP_H
#define RELIC_EP_H

#include "relic_fp.h"
#include "relic_bn.h"
#include "relic_types.h"

/**
 * MIRACL's Barreto-Naehrig pairing-friendly prime elliptic curve
 */
#define RATE_P256		0

/**
 * Optimization identifier for the case a = 0.
 */
#define EP_OPT_ZERO		0

/**
 * Optimization identifier for the case a = 1.
 */
#define EP_OPT_ONE		1

/**
 * Optimization identifier for the case when a is small.
 */
#define EP_OPT_DIGIT	2

/**
 * Optimization identifier for the general case when a is big.
 *
 */
#define EP_OPT_NONE		3

/**
 * Represents an ellyptic curve point over a prime field.
 */
typedef struct {
#if ALLOC == STATIC
	/** The first coordinate. */
	fp_t x;
	/** The second coordinate. */
	fp_t y;
	/** The third coordinate (projective representation). */
	fp_t z;
#elif ALLOC == DYNAMIC || ALLOC == STACK
	/** The first coordinate. */
	align fp_st x;
	/** The second coordinate. */
	align fp_st y;
	/** The third coordinate (projective representation). */
	align fp_st z;
#endif
	/** Flag to indicate that this point is normalized. */
	int norm;
} ep_st;

/**
 * Pointer to an elliptic curve point.
 */
typedef ep_st *ep_t;

/**
 * Initializes the prime elliptic curve arithmetic module.
 */
void ep_curve_init(void);

/**
 * Finalizes the prime elliptic curve arithmetic module.
 */
void ep_curve_clean(void);

/**
 * Returns the a coefficient of the currently configured prime elliptic curve.
 *
 * @return the a coefficient of the elliptic curve.
 */
dig_t *ep_curve_get_a(void);

/**
 * Returns the b coefficient of the currently configured prime elliptic curve.
 *
 * @return the b coefficient of the elliptic curve.
 */
dig_t *ep_curve_get_b(void);

/**
 * Returns a optimization identifier based on the coefficient a of the curve.
 *
 * @return the optimization identifier.
 */
int ep_curve_opt_a(void);

/**
 * Tests if the configured prime elliptic curve is supersingular.
 *
 * @return 1 if the prime elliptic curve is supersingular, 0 otherwise.
 */
int ep_curve_is_super(void);

/**
 * Returns the generator of the group of points in the prime elliptic curve.
 *
 * @param[out] g			- the point to store the generator.
 */
ep_t ep_curve_get_gen(void);

/**
 * Returns the order of the group of points in the prime elliptic curve.
 *
 * @param[out] r			- the multiple precision integer to store the order.
 */
bn_t ep_curve_get_ord(void);

/**
 * Configures a new ordinary prime elliptic curve by its coefficients.
 *
 * @param[in] a			- the coefficient a of the curve.
 * @param[in] b			- the coefficient b of the curve.
 */
void ep_curve_set_ordin(fp_t a, fp_t b);

/**
 * Configures a new generator for the group of points in the prime elliptic
 * curve.
 *
 * @param[in] g			- the new generator.
 */
void ep_curve_set_gen(ep_t g);

/**
 * Changes the order of the group of points.
 *
 * @param[in] r			- the new order.
 */
void ep_curve_set_ord(bn_t r);

/**
 * Configures a new prime elliptic curve by its parameter identifier.
 *
 * @param				- the parameters identifier.
 */
void ep_param_set(int param);

/**
 * Calls a function to allocate a point on a prime elliptic curve.
 *
 * @param[out] A			- the new point.
 * @throw ERR_NO_MEMORY		- if there is no available memory.
 */
#if ALLOC == DYNAMIC
#define ep_new(A)															\
	A = (ep_t)calloc(1, sizeof(ep_st));										\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\

#elif ALLOC == STATIC
#define ep_new(A)															\
	A = (ep_st *)alloca(sizeof(ep_st));										\
	fp_new((A)->x);															\
	fp_new((A)->y);															\
	fp_new((A)->z);															\

#elif ALLOC == STACK
#define ep_new(A)															\
	A = (ep_t)alloca(sizeof(ep_st));										\

#endif

/**
 * Calls a function to clean and free a point on a prime elliptic curve.
 *
 * @param[out] A			- the point to free.
 */
#if ALLOC == DYNAMIC
#define ep_free(A)															\
	if (A != NULL) {														\
		free(A);															\
		A = NULL;															\
	}

#elif ALLOC == STATIC
#define ep_free(A)															\
	if (A != NULL) {														\
		fp_free((A)->x);													\
		fp_free((A)->y);													\
		fp_free((A)->z);													\
		A = NULL;															\
	}																		\

#elif ALLOC == STACK
#define ep_free(A)															\
	A = NULL;																\

#endif

/**
 * Initializes a previously allocated prime elliptic curve point.
 *
 * @param[out] a			- the point to initialize.
 * @param[in] digits		- the required precision in digits.
 * @throw ERR_NO_MEMORY		- if there is no available memory.
 * @throw ERR_PRECISION		- if the required precision cannot be represented
 * 							by the library.
 */
void ep_init(ep_t a);

/**
 * Cleans an prime elliptic curve point..
 *
 * @param[out] a			- the point to free.
 */
void ep_clean(ep_t a);

/**
 * Tests if a point on a prime elliptic curve is at the infinity.
 *
 * @param[in] p				- the point to test.
 * @return 1 if the point is at infinity, 0 otherise.
 */
int ep_is_infty(ep_t p);

/**
 * Assigns a prime elliptic curve point to a point at the infinity.
 *
 * @param[out] p			- the point to assign.
 */
void ep_set_infty(ep_t p);

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] q			- the result.
 * @param[in] p				- the prime elliptic curve point to copy.
 */
void ep_copy(ep_t r, ep_t p);

/**
 * Compares two prime elliptic curve points.
 *
 * @param[in] p				- the first prime elliptic curve point.
 * @param[in] q				- the second prime elliptic curve point.
 * @return CMP_EQ if p == q and CMP_NE if p != q.
 */
int ep_cmp(ep_t p, ep_t q);

/**
 * Assigns a random value to a prime elliptic curve point.
 *
 * @param[out] p			- the prime elliptic curve point to assign.
 */
void ep_rand(ep_t p);

/**
 * Prints a prime elliptic curve point.
 *
 * @param[in] p				- the prime elliptic curve point to print.
 */
void ep_print(ep_t p);

/**
 * Negates a prime elliptic curve point.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the point to negate.
 */
#if EP_ADD == BASIC
#define ep_neg(R, P)		ep_neg_basic(R, P)
#elif EP_ADD == PROJC
#define ep_neg(R, P)		ep_neg_projc(R, P)
#endif

/**
 * Negates a prime elliptic curve point represented by affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to negate.
 */
void ep_neg_basic(ep_t r, ep_t p);

/**
 * Negates a prime elliptic curve point represented by projective coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to negate.
 */
void ep_neg_projc(ep_t r, ep_t p);

/**
 * Adds two prime elliptic curve points.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first point to add.
 * @param[in] Q				- the second point to add.
 */
#if EP_ADD == BASIC
#define ep_add(R, P, Q)		ep_add_basic(R, P, Q);
#elif EP_ADD == PROJC
#define ep_add(R, P, Q)		ep_add_projc(R, P, Q);
#endif

/**
 * Adds two prime elliptic curve points represented in affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to add.
 * @param[in] q				- the second point to add.
 */
void ep_add_basic(ep_t r, ep_t p, ep_t q);

/**
 * Adds two prime elliptic curve points represented in projective coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point to add.
 * @param[in] q				- the second point to add.
 */
void ep_add_projc(ep_t r, ep_t p, ep_t q);

/**
 * Subtracts a prime elliptic curve point from another, that is, compute
 * R = P - Q.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first point.
 * @param[in] Q				- the second point.
 */
#if EP_ADD == BASIC
#define ep_sub(R, P, Q)		ep_sub_basic(R, P, Q)
#elif EP_ADD == PROJC
#define ep_sub(R, P, Q)		ep_sub_projc(R, P, Q)
#endif

/**
 * Subtracts a prime elliptic curve point from another, both points represented
 * by affine coordinates..
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point.
 * @param[in] q				- the second point.
 */
void ep_sub_basic(ep_t r, ep_t p, ep_t q);

/**
 * Subtracts a prime elliptic curve point from another, both points represented
 * by projective coordinates..
 *
 * @param[out] r			- the result.
 * @param[in] p				- the first point.
 * @param[in] q				- the second point.
 */
void ep_sub_projc(ep_t r, ep_t p, ep_t q);

/**
 * Doubles a prime elliptic curve point.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the point to double.
 */
#if EP_ADD == BASIC
#define ep_dbl(R, P)		ep_dbl_basic(R, P);
#elif EP_ADD == PROJC
#define ep_dbl(R, P)		ep_dbl_projc(R, P);
#endif

/**
 * Doubles a prime elliptic curve point represented in affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to double.
 */
void ep_dbl_basic(ep_t r, ep_t p);

/**
 * Doubles a prime elliptic curve point represented in projective coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to double.
 */
void ep_dbl_projc(ep_t r, ep_t p);

/**
 * Multiplies a prime elliptic curve point by an integer.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the point to multiply.
 * @param[in] K				- the integer.
 */
#if EP_MUL == BASIC
#define ep_mul(R, P, K)		ep_mul_basic(R, P, K)
#elif EP_MUL == CONST
#define ep_mul(R, P, K)		ep_mul_const(R, P, K)
#elif EP_MUL == W4NAF
#define ep_mul(R, P, K)		ep_mul_w4naf(R, P, K)
#elif EP_MUL == W5NAF
#define ep_mul(R, P, K)		ep_mul_w5naf(R, P, K)
#endif

/**
 * Multiplies a prime elliptic point by an integer using the Binary method.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to multiply.
 * @param[in] k				- the integer.
 */
void ep_mul_basic(ep_t r, ep_t p, bn_t k);

/**
 * Converts a point to affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to convert.
 */
void ep_norm(ep_t r, ep_t p);

#endif /* !RELIC_EP_H */
