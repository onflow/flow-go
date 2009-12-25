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
 * @defgroup pp Pairings over prime elliptic curves.
 */

/**
 * @file
 *
 * Interface of the bilinear pairing over prime elliptic curves functions.
 *
 * @version $Id$
 * @ingroup pp
 */

#ifndef RELIC_PP_H
#define RELIC_PP_H

#include "relic_fp.h"
#include "relic_ep.h"
#include "relic_types.h"

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a quadratic extension prime field element.
 *
 * This extension is constructed with the basis {1, u}, where u is an
 * adjoined square root in the prime field. If p = 3 mod 8, u^2 = -1; otherwise
 * u^2 = -2.
 */
typedef fp_t fp2_t[2];

/**
 * Represents a quadratic extension field element with automatic memory
 * allocation.
 */
typedef fp_st fp2_st[2];

/**
 * Represents a sextic extension field element.
 *
 * This extension is constructed with the basis {1, v, v^2}, where v is an
 * adjoined cube root in the underlying quadratic extension.  If p = 3 mod 8,
 * v^3 = u + 1; if p = 5 mod 8, v^3 = u; if p = 7 mod 8 and p = 2,3 mod 5,
 * v^3 = u + 2.
 */
typedef fp2_t fp6_t[3];

/**
 * Represents a dodecic extension field element.
 *
 * This extension is constructed with the basis {1, w}, where w^2 = v is an
 * adjoined square root in the underlying sextic extension.
 */
typedef fp6_t fp12_t[2];

/**
 * Represents an elliptic curve point over a quadratic extension over a prime
 * field.
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

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Initializes a quadratic extension field element with a null value.
 *
* @param[out] A			- the quadratic extension element to initialize.
 */
#define fp2_null(A)															\
		fp_null(A[0]); fp_null(A[1]);										\

/**
 * Allocate and initializes a quadratic extension field element.
 *
 * @param[out] A			- the new quadratic extension field element.
 */
#define fp2_new(A)															\
		fp_new(A[0]); fp_new(A[1]);											\

/**
 * Calls a function to clean and free a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to free.
 */
#define fp2_free(A)															\
		fp_free(A[0]); fp_free(A[1]); 										\

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element to copy.
 */
#define fp2_copy(C, A)														\
		fp_copy(C[0], A[0]); fp_copy(C[1], A[1]); 							\

/**
 * Negates a quadratic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the quadratic extension field element to negate.
 */
#define fp2_neg(C, A)														\
		fp_neg(C[0], A[0]); fp_neg(C[1], A[1]); 							\

/**
 * Assigns zero to a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to zero.
 */
#define fp2_zero(A)															\
		fp_zero(A[0]); fp_zero(A[1]); 										\

/**
 * Tests if a quadratic extension field element is zero or not.
 *
 * @param[in] A				- the quadratic extension field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
#define fp2_is_zero(A)														\
		(fp_is_zero(A[0]) && fp_is_zero(A[1])) 								\

/**
 * Assigns a random value to a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to assign.
 */
#define fp2_rand(A)															\
		fp_rand(A[0]); fp_rand(A[1]);										\

/**
 * Prints a quadratic extension field element to standard output.
 *
 * @param[in] A				- the quadratic extension field element to print.
 */
#define fp2_print(A)														\
		fp_print(A[0]); fp_print(A[1]);										\

/**
 * Returns the result of a comparison between two quadratic extension field
 * elements
 *
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 * @return CMP_LT if a < b, CMP_EQ if a == b and CMP_GT if a > b.
 */
#define fp2_cmp(A, B)														\
		((fp_cmp(A[0], B[0]) == CMP_EQ) && (fp_cmp(A[1], B[1]) == CMP_EQ)	\
		? CMP_EQ : CMP_NE)													\

/**
 * Adds two quadratic extension field elements. Computes C = A + B.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 */
#define fp2_add(C, A, B)													\
		fp_add(C[0], A[0], B[0]); fp_add(C[1], A[1], B[1]);					\

/**
 * Subtracts a quadratic extension field element from another. Computes
 * C = A - B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element.
 * @param[in] B				- the quadratic extension field element.
 */
#define fp2_sub(C, A, B)													\
		fp_sub(C[0], A[0], B[0]); fp_sub(C[1], A[1], B[1]);					\

/**
 * Initializes a sextic extension field with a null value.
 *
 * @param[out] A			- the sextic extension element to initialize.
 */
#define fp6_null(A)															\
		fp2_null(A[0]); fp2_null(A[1]); fp2_null(A[2]);						\

/**
 * Allocate and initializes a sextic extension field element.
 *
 * @param[out] A			- the new sextic extension field element.
 */
#define fp6_new(A)															\
		fp2_new(A[0]); fp2_new(A[1]); fp2_new(A[2]);						\

/**
 * Calls a function to clean and free a sextic extension field element.
 *
 * @param[out] A			- the sextic extension field element to free.
 */
#define fp6_free(A)															\
		fp2_free(A[0]); fp2_free(A[1]); fp2_new(A[2]); 						\

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the sextic extension field element to copy.
 */
#define fp6_copy(C, A)														\
		fp2_copy(C[0], A[0]); fp2_copy(C[1], A[1]); fp2_copy(C[2], A[2]);	\

/**
 * Negates a sextic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the sextic extension field element to negate.
 */
#define fp6_neg(C, A)														\
		fp2_neg(C[0], A[0]); fp2_neg(C[1], A[1]); fp2_neg(C[2], A[2]); 		\

/**
 * Assigns zero to a sextic extension field element.
 *
 * @param[out] A			- the sextic extension field element to zero.
 */
#define fp6_zero(A)															\
		fp2_zero(A[0]); fp2_zero(A[1]); fp2_zero(A[2]); 					\

/**
 * Tests if a sextic extension field element is zero or not.
 *
 * @param[in] A				- the sextic extension field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
#define fp6_is_zero(A)														\
		(fp2_is_zero(A[0]) && fp2_is_zero(A[1]) && fp2_is_zero(A[2]))		\

/**
 * Assigns a random value to a sextic extension field element.
 *
 * @param[out] A			- the sextic extension field element to assign.
 */
#define fp6_rand(A)															\
		fp2_rand(A[0]); fp2_rand(A[1]); fp2_rand(A[2]);						\

/**
 * Prints a sextic extension field element to standard output.
 *
 * @param[in] A				- the sextic extension field element to print.
 */
#define fp6_print(A)														\
		fp2_print(A[0]); fp2_print(A[1]); fp2_print(A[2]);					\

/**
 * Returns the result of a comparison between two sextic extension field
 * elements
 *
 * @param[in] A				- the first sextic extension field element.
 * @param[in] B				- the second sextic extension field element.
 * @return CMP_LT if a < b, CMP_EQ if a == b and CMP_GT if a > b.
 */
#define fp6_cmp(A, B)														\
		((fp2_cmp(A[0], B[0]) == CMP_EQ) && (fp2_cmp(A[1], B[1]) == CMP_EQ)	\
		&& (fp2_cmp(A[2], B[2]) == CMP_EQ) ? CMP_EQ : CMP_NE)				\

/**
 * Adds two sextic extension field elements. Computes C = A + B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first sextic extension field element.
 * @param[in] B				- the second sextic extension field element.
 */
#define fp6_add(C, A, B)													\
		fp2_add(C[0], A[0], B[0]); fp2_add(C[1], A[1], B[1]);				\
		fp2_add(C[2], A[2], B[2]);											\

/**
 * Subtracts a sextic extension field element from another, that is, computes
 * C = A - B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the sextic extension field element.
 * @param[in] B				- the sextic extension field element.
 */
#define fp6_sub(C, A, B)													\
		fp2_sub(C[0], A[0], B[0]); fp2_sub(C[1], A[1], B[1]);				\
		fp2_sub(C[2], A[2], B[2]);											\


/**
 * Initializes a dodecic extension field with a null value.
 *
 * @param[out] A			- the dodecic extension element to initialize.
 */
#define fp12_null(A)														\
		fp6_null(A[0]); fp6_null(A[1]);										\

/**
 * Allocate and initializes a dodecic extension field element.
 *
 * @param[out] A			- the new dodecic extension field element.
 */
#define fp12_new(A)															\
		fp6_new(A[0]); fp6_new(A[1]);										\

/**
 * Calls a function to clean and free a dodecic extension field element.
 *
 * @param[out] A			- the dodecic extension field element to free.
 */
#define fp12_free(A)														\
		fp6_free(A[0]); fp6_free(A[1]); 									\

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the dodecic extension field element to copy.
 */
#define fp12_copy(C, A)														\
		fp6_copy(C[0], A[0]); fp6_copy(C[1], A[1]); 						\

/**
 * Negates a dodecic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the dodecic extension field element to negate.
 */
#define fp12_neg(C, A)														\
		fp6_neg(C[0], A[0]); fp6_neg(C[1], A[1]); 							\

/**
 * Assigns zero to a dodecic extension field element.
 *
 * @param[out] A			- the dodecic extension field element to zero.
 */
#define fp12_zero(A)														\
		fp6_zero(A[0]); fp6_zero(A[1]); 									\

/**
 * Tests if a dodecic extension field element is zero or not.
 *
 * @param[in] A				- the dodecic extension field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
#define fp12_is_zero(A)														\
		fp6_is_zero(A[0]) || fp6_is_zero(A[1]) 								\

/**
 * Assigns a random value to a dodecic extension field element.
 *
 * @param[out] A			- the dodecic extension field element to assign.
 */
#define fp12_rand(A)														\
		fp6_rand(A[0]); fp6_rand(A[1]);										\

/**
 * Prints a dodecic extension field element to standard output.
 *
 * @param[in] A				- the dodecic extension field element to print.
 */
#define fp12_print(A)														\
		fp6_print(A[0]); fp6_print(A[1]);									\

/**
 * Returns the result of a comparison between two dodecic extension field
 * elements
 *
 * @param[in] a				- the first dodecic extension field element.
 * @param[in] b				- the second dodecic extension field element.
 * @return CMP_LT if a < b, CMP_EQ if a == b and CMP_GT if a > b.
 */
#define fp12_cmp(A, B)														\
		((fp6_cmp(A[0], B[0]) == CMP_EQ) && (fp6_cmp(A[1], B[1]) == CMP_EQ)	\
		? CMP_EQ : CMP_NE)													\

/**
 * Adds two dodecic extension field elements. Computes C = A + B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first dodecic extension field element.
 * @param[in] B				- the second dodecic extension field element.
 */
#define fp12_add(C, A, B)													\
		fp6_add(C[0], A[0], B[0]); fp6_add(C[1], A[1], B[1]);				\

/**
 * Subtracts a dodecic extension field element from another. Computes
 * C = A - B.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first dodecic extension field element.
 * @param[in] B				- the second dodecic extension field element.
 */
#define fp12_sub(C, A, B)													\
		fp6_sub(C[0], A[0], B[0]); fp6_sub(C[1], A[1], B[1]);				\

/**
 * Initializes a point on a prime elliptic curve with a null value.
 *
 * @param[out] A			- the point to initialize.
 */
#define ep2_null(A)		A = NULL;

/**
 * Calls a function to allocate a point on a prime elliptic curve.
 *
 * @param[out] A			- the new point.
 * @throw ERR_NO_MEMORY		- if there is no available memory.
 */
#if ALLOC == DYNAMIC
#define ep2_new(A)															\
	A = (ep2_t)calloc(1, sizeof(ep2_st));									\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	fp2_new((A)->x);														\
	fp2_new((A)->y);														\
	fp2_new((A)->z);														\

#elif ALLOC == STATIC
#define ep2_new(A)															\
	A = (ep2_st *)alloca(sizeof(ep2_st));									\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
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
		free(A);															\
		A = NULL;															\
	}

#elif ALLOC == STATIC
#define ep2_free(A)															\
	if (A != NULL) {														\
		fp2_free((A)->x);													\
		fp2_free((A)->y);													\
		fp2_free((A)->z);													\
		A = NULL;															\
	}																		\

#elif ALLOC == STACK
#define ep2_free(A)															\
	A = NULL;																\

#endif

/**
 * Adds two prime elliptic curve points. Computes R = P + Q.
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
 * Adds two prime elliptic curve points returning the slope.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the first point to add.
 * @param[in] Q				- the second point to add.
 */
#if EP_ADD == BASIC
#define ep2_add_slp(R, S, P, Q)	ep2_add_slp_basic(R, S, P, Q);
#elif EP_ADD == PROJC
#define ep2_add_slp(R, S, P, Q)	ep2_add_slp_projc(R, S, P, Q);
#endif

/**
 * Doubles a prime elliptic curve point. Computes R = 2 * P.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the point to double.
 */
#if EP_ADD == BASIC
#define ep2_dbl(R, P)			ep2_dbl_basic(R, P);
#elif EP_ADD == PROJC
#define ep2_dbl(R, P)			ep2_dbl_basic(R, P);
#endif

/**
 * Doubles a prime elliptic curve point returning the slope.
 *
 * @param[out] R			- the result.
 * @param[in] P				- the point to double.
 */
#if EP_ADD == BASIC
#define ep2_dbl_slp(R, S, E, P)	ep2_dbl_slp_basic(R, S, E, P);
#elif EP_ADD == PROJC
#define ep2_dbl_slp(R, S, E, P)	ep2_dbl_slp_projc(R, S, E, P);
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Doubles a quadratic extension field element. Computes c = 2 * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to double.
 */
void fp2_dbl(fp2_t c, fp2_t a);

/**
 * Multiples two quadratic extension field elements, that is, compute c = a * b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element.
 * @param[in] b				- the quadratic extension field element.
 */
void fp2_mul(fp2_t c, fp2_t a, fp2_t b);

/**
 * Multiplies a quadratic extension field element by the adjoined square root.
 * Computes c = a * u.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to multiply.
 */
void fp2_mul_art(fp2_t c, fp2_t a);

/**
 * Multiplies a quadratic extension field element by a quadratic/cubic
 * non-residue. Computes c = a * v, where v is a non-square/non-cube in the
 * quadratic extension.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to multiply.
 */
void fp2_mul_nor(fp2_t c, fp2_t a);

/**
 * Computes the square of a quadratic extension field element. Computes
 * c = a * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to square.
 */
void fp2_sqr(fp2_t c, fp2_t a);

/**
 * Inverts a quadratic extension field element. Computes c = a^(-1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to invert.
 */
void fp2_inv(fp2_t c, fp2_t a);

/**
 * Computes the Frobenius of a quadratic extension field element. This is the
 * same as computing the conjugate of the extension field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension element to conjugate.
 */
void fp2_frb(fp2_t c, fp2_t a);

/**
 * Doubles a sextic extension field element. Computes c = 2 * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to double.
 */
void fp6_dbl(fp6_t c, fp6_t a);

/**
 * Multiples two sextic extension field elements. Computes c = a * b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element.
 * @param[in] b				- the sextic extension field element.
 */
void fp6_mul(fp6_t c, fp6_t a, fp6_t b);

/**
 * Multiplies a sextic extension field element by the adjoined cube root.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to multiply.
 */
void fp6_mul_art(fp6_t c, fp6_t a);

/**
 * Multiples a sextic extension field element by a quadratic extension
 * field element.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element.
 * @param[in] b				- the quadratic extension field element.
 */
void fp6_mul_dexqu(fp6_t c, fp6_t a, fp2_t b);

/**
 * Multiples a dense sextic extension field element by a sparse element.
 *
 * The sparse element must have a[2] = 0.
 *
 * @param[out] c			- the result.
 * @param[in] a				- a sextic extension field element.
 * @param[in] b				- a sparse sextic extension field element.
 */
void fp6_mul_dexsp(fp6_t c, fp6_t a, fp6_t b);

/**
 * Computes the square of a sextic extension field element. Computes c = a * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to square.
 */
void fp6_sqr(fp6_t c, fp6_t a);

/**
 * Inverts a sextic extension field element. Computes c = a^(-1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the sextic extension field element to invert.
 */
void fp6_inv(fp6_t c, fp6_t a);

/**
 * Computes the Frobenius endomorphism of a unitary sextic extension
 * field element. Computes c = a^p.
 *
 * @param[out] c			- the result.
 * @param[in] a				- a sextic extension field element.
 * @param[in] b				- the constant v^p used for the computation.
 */
void fp6_frb(fp6_t c, fp6_t a, fp6_t b);

/**
 * Computes the inverse of a dodecic unitary extension field element.
 *
 * For unitary elements, this is equivalent to computing the conjugate.
 * A unitary element is one previously raised to the (p^6 - 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element to conjugate.
 */
void fp12_conj(fp12_t c, fp12_t a);

/**
 * Multiples two dodecic extension field elements. Compute c = a * b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first dodecic extension field element.
 * @param[in] b				- the second dodecic extension field element.
 */
void fp12_mul(fp12_t c, fp12_t a, fp12_t b);

/**
 * Multiples a dense dodecic extension field element by a sparse element.
 *
 * The sparse element must have only the a[0][0], a[1][0] and a[1][1] elements
 * not equal to zero.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dense dodecic extension field element.
 * @param[in] b				- the sparse dodecic extension field element.
 */
void fp12_mul_dexsp(fp12_t c, fp12_t a, fp12_t b);

/**
 * Computes the square of a dodecic extension field element. Computes
 * c = a * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element to square.
 */
void fp12_sqr(fp12_t c, fp12_t a);

/**
 * Computes the square of a unitary dodecic extension field element.
 *
 * A unitary element is one previously raised to the (p^6 - 1)-th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the unitary dodecic extension element to square.
 */
void fp12_sqr_uni(fp12_t c, fp12_t a);

/**
 * Inverts a dodecic extension field element. Computes c = a^(-1).
 *
 * @param[out] c			- the result.
 * @param[in] a				- the dodecic extension field element to invert.
 */
void fp12_inv(fp12_t c, fp12_t a);

/**
 * Computes the Frobenius endomorphism of a dodecic extension element.
 * Computes c = a^p.
 *
 * @param[out] c			- the result.
 * @param[in] a				- a dodecic extension field element.
 * @param[in] b				- the constant w^p used for the computation.
 */
void fp12_frb(fp12_t c, fp12_t a, fp12_t b);

/**
 * Initializes the elliptic curve over quadratic extension.
 */
void ep2_curve_init(void);

/**
 * Finalizes the elliptic curve over quadratic extension.
 */
void ep2_curve_clean(void);

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

void ep2_curve_set_twist(int twist);

/**
 * Computes a power of a unitary dodecic extension field element.
 *
 * A unitary element is one previously raised to the (p^6 - 1)th power.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the unitary dodecic extension field element.
 * @param[in] b				- the power.
 */
void fp12_exp_uni(fp12_t c, fp12_t a, bn_t b);

/**
 * Tests if a point on a prime elliptic curve is at the infinity.
 *
 * @param[in] p				- the point to test.
 * @return 1 if the point is at infinity, 0 otherise.
 */
int ep2_is_infty(ep2_t p);

/**
 * Assigns a prime elliptic curve point to a point at the infinity.
 *
 * @param[out] p			- the point to assign.
 */
void ep2_set_infty(ep2_t p);

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
 * Converts a point to affine coordinates.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to convert.
 */
void ep2_norm(ep2_t r, ep2_t p);

/**
 * Initializes the pairing over prime fields module.
 */
void pp_core_init(void);

/**
 * Finalizes the pairing over prime fields module.
 */
void pp_core_clean(void);

/**
 * Computes the R-ate pairing of two points in a parameterized elliptic curve.
 * Computes e(Q, P).
 * 
 * @param[out] r			- the result.
 * @param[in] q				- the first elliptic curve point.
 * @param[in] p				- the second elliptic curve point.
 * @param[in] x				- the parameter used to generate the elliptic curve.
 * @param[in] f				- the constant w^p used in the Frobenius.
 */
void pp_pair_rate(fp12_t r, ep2_t q, ep_t p, bn_t x, fp12_t f);

#endif /* !RELIC_PP_H */
