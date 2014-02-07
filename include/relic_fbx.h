/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2014 RELIC Authors
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
 * @defgroup fbx Extensions of binary fields
 */

/**
 * @file
 *
 * Interface of the module for extension field arithmetic over binary fields.
 *
 * @version $Id$
 * @ingroup fbx
 */

#ifndef RELIC_FBX_H
#define RELIC_FBX_H

#include "relic_fb.h"
#include "relic_types.h"

/*============================================================================*/
/* Type definitions                                                           */
/*============================================================================*/

/**
 * Represents a quadratic extension binary field element.
 *
 * This extension field is constructed with the basis {1, s}, where s is a
 * quadratic non-residue in the binary field.
 */
typedef fb_t fb2_t[2];

/**
 * Represents a quartic extension binary field element.
 *
 * This extension field is constructed with the basis {1, s, t, st}, where s is
 * a quadratic non-residue in the binary field and t is a quadratic non-residue
 * in the quadratic extension field (s^2 = s + 1 and t^2 = t + 1).
 */
typedef fb_t fb4_t[4];

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Initializes a quadratic extension binary field with a null value.
 *
 * @param[out] A			- the quadratic extension element to initialize.
 */
#define fb2_null(A)															\
		fb_null(A[0]); fb_null(A[1]);										\

/**
 * Calls a function to allocate a quadratic extension binary field element.
 *
 * @param[out] A			- the new quadratic extension field element.
 */
#define fb2_new(A)															\
		fb_new(A[0]); fb_new(A[1]);											\

/**
 * Calls a function to free a quadratic extension binary field element.
 *
 * @param[out] A			- the quadratic extension field element to free.
 */
#define fb2_free(A)															\
		fb_free(A[0]); fb_free(A[1]); 										\

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element to copy.
 */
#define fb2_copy(C, A)														\
		fb_copy(C[0], A[0]); fb_copy(C[1], A[1]); 							\

/**
 * Negates a quadratic extension field element.
 *
 * f@param[out] C			- the result.
 * @param[out] A			- the quadratic extension field element to negate.
 */
#define fb2_neg(C, A)														\
		fb_neg(C[0], A[0]); fb_neg(C[1], A[1]); 							\

/**
 * Assigns zero to a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to zero.
 */
#define fb2_zero(A)															\
		fb_zero(A[0]); fb_zero(A[1]); 										\

/**
 * Tests if a quadratic extension field element is zero or not.
 *
 * @param[in] A				- the quadratic extension field element to test.
 * @return 1 if the argument is zero, 0 otherwise.
 */
#define fb2_is_zero(A)														\
		(fb_is_zero(A[0]) && fb_is_zero(A[1]))								\

/**
 * Assigns a random value to a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to assign.
 */
#define fb2_rand(A)															\
		fb_rand(A[0]); fb_rand(A[1]);										\

/**
 * Prints a quadratic extension field element to standard output.
 *
 * @param[in] A				- the quadratic extension field element to print.
 */
#define fb2_print(A)														\
		fb_print(A[0]); fb_print(A[1]);										\

/**
 * Returns the result of a comparison between two quadratic extension field
 * elements
 *
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 * @return CMP_NE if a != b, CMP_EQ if a == b.
 */
#define fb2_cmp(A, B)														\
		((fb_cmp(A[0], B[0]) == CMP_EQ) && (fb_cmp(A[1], B[1]) == CMP_EQ)	\
		? CMP_EQ : CMP_NE)													\

/**
 * Adds two quadratic extension field elements. Computes c = a + b.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first quadratic extension field element.
 * @param[in] B				- the second quadratic extension field element.
 */
#define fb2_add(C, A, B)													\
		fb_add(C[0], A[0], B[0]); fb_add(C[1], A[1], B[1]);					\

/**
 * Subtracts a quadratic extension field element from another. Computes
 * c = a - b.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension binary field element.
 * @param[in] B				- the quadratic extension binary field element.
 */
#define fb2_sub(C, A, B)													\
		fb_sub(C[0], A[0], B[0]); fb_sub(C[1], A[1], B[1]);					\

/**
 * Initializes a quadratic extension binary field with a null value.
 *
 * @param[out] A			- the quartic extension element to initialize.
 */
#define fb4_null(A)															\
		fb_null(A[0]); fb_null(A[1]); fb_null(A[2]); fb_null(A[3]);			\

/**
 * Calls a funtion to allocate a quartic extension binary field element.
 *
 * @param[out] A			- the new quartic extension field element.
 */
#define fb4_new(A)															\
		fb_new(A[0]); fb_new(A[1]); fb_new(A[2]); fb_new(A[3]);				\

/**
 * Calls a function to free a quartic extension binary field element.
 *
 * @param[out] A			- the quartic extension field element to free.
 */
#define fb4_free(A)															\
		fb_free(A[0]); fb_free(A[1]); fb_free(A[2]); fb_free(A[3]);			\

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quartic extension field element to copy.
 */
#define fb4_copy(C, A)														\
		fb_copy(C[0], A[0]); fb_copy(C[1], A[1]); 							\
		fb_copy(C[2], A[2]); fb_copy(C[3], A[3]);							\

/**
 * Negates a quartic extension field element.
 *
 * @param[out] C			- the result.
 * @param[out] A			- the quartic extension field element to negate.
 */
#define fb4_neg(C, A)														\
		fb_neg(C[0], A[0]); fb_neg(C[1], A[1]); 							\
		fb_neg(C[2], A[2]); fb_neg(C[3], A[3]);								\

/**
 * Assigns zero to a quartic extension field element.
 *
 * @param[out] A			- the quartic extension field element to zero.
 */
#define fb4_zero(A)															\
		fb_zero(A[0]); fb_zero(A[1]); fb_zero(A[2]); fb_zero(A[3]);			\

/**
 * Assigns zero to a quartic extension field element.
 *
 * @param[out] C			- the quartic extension field element to assign.
 * @param[in] A				- the digit.
 */
#define fb4_set_dig(C, A)													\
		fb_set_dig(C[0], A); fb_zero(C[1]); fb_zero(C[2]); fb_zero(C[3]);	\

/**
 * Tests if a quartic extension field element is zero or not.
 *
 * @param[in] A				- the quartic extension field element to test.
 * @return 1 if the argument is zero, 0 otherwise.
 */
#define fb4_is_zero(A)														\
		(fb_is_zero(A[0]) && fb_is_zero(A[1]) &&							\
		fb_is_zero(A[2]) && fb_is_zero(A[3]))								\

/**
 * Assigns a random value to a quartic extension field element.
 *
 * @param[out] A			- the binary field element to assign.
 */
#define fb4_rand(A)															\
		fb_rand(A[0]); fb_rand(A[1]); fb_rand(A[2]); fb_rand(A[3]);			\

/**
 * Prints a quartic extension field element to standard output.
 *
 * @param[in] A				- the quartic extension field element to print.
 */
#define fb4_print(A)														\
		fb_print(A[0]); fb_print(A[1]); fb_print(A[2]); fb_print(A[3]);		\

/**
 * Returns the result of a comparison between two quartic extension field
 * elements.
 *
 * @param[in] A				- the first quartic extension field element.
 * @param[in] B				- the second quartic extension field element.
 * @return CMP_NE if a != b, CMP_EQ if a == b.
 */
#define fb4_cmp(A, B)														\
		((fb_cmp(A[0], B[0]) == CMP_EQ) && (fb_cmp(A[1], B[1]) == CMP_EQ) &&\
		(fb_cmp(A[2], B[2]) == CMP_EQ) && (fb_cmp(A[3], B[3]) == CMP_EQ)	\
		? CMP_EQ : CMP_NE)													\

/**
 * Returns the result of a comparison between a quartic extension field
 * and a digit.
 *
 * @param[in] A				- the quartic extension field element.
 * @param[in] B				- the digit.
 * @return CMP_NE if a != b, CMP_EQ if a == b.
 */
#define fb4_cmp_dig(A, B)													\
		((fb_cmp_dig(A[0], B) == CMP_EQ) && (fb_is_zero(A[1]) == CMP_EQ) &&	\
		(fb_is_zero(A[2]) == CMP_EQ) && (fb_is_zero(A[3]) == CMP_EQ)		\
		? CMP_EQ : CMP_NE)													\

/**
 * Adds two quartic extension field elements. Computes c = a + b.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first binary field element to add.
 * @param[in] B				- the second binary field element to add.
 */
#define fb4_add(C, A, B)													\
		fb_add(C[0], A[0], B[0]); fb_add(C[1], A[1], B[1]);					\
		fb_add(C[2], A[2], B[2]); fb_add(C[3], A[3], B[3]);					\

/**
 * Subtracts a quartic extension field element from another. Computes
 * c = a - b.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the first quartic extension field element.
 * @param[in] B				- the second quartic extension field element.
 */
#define fb4_sub(C, A, B)													\
		fb_sub(C[0], A[0], B[0]); fb_sub(C[1], A[1], B[1]);					\
		fb_sub(C[2], A[2], B[2]); fb_sub(C[3], A[3], B[3]);					\

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Multiples two quadratic extension field elements. Computes c = a * b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension binary field element.
 * @param[in] b				- the quadratic extension binary field element.
 */
void fb2_mul(fb2_t c, fb2_t a, fb2_t b);

 /**
  * Multiples a quadratic extension field element by a quadratic non-residue.
  * Computes c = a * s.
  *
  * @param[out] c			- the result.
  * @param[in] a				- the quadratic extension binary field element.
  * @param[in] b				- the quadratic extension binary field element.
  */
 void fb2_mul_nor(fb2_t c, fb2_t a);

/**
 * Computes the square of a quadratic extension field element. Computes
 * c = a * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the binary field element to square.
 */
void fb2_sqr(fb2_t c, fb2_t a);

/**
 * Solves a quadratic equation for c, Tr(a) = 0. Computes c such that
 * c^2 + c = a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element.
 */
void fb2_slv(fb2_t c, fb2_t a);

/**
 * Inverts a quadratic extension field element. Computes c = a^{-1}.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quadratic extension field element to invert.
 */
void fb2_inv(fb2_t c, fb2_t a);

/**
 * Multiples two quartic extension field elements. Computes c = a * b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quartic extension field element.
 * @param[in] b				- the second quartic extension field element.
 */
void fb4_mul(fb4_t c, fb4_t a, fb4_t b);

/**
 * Multiplies two quartic extension field elements represented in a polynomial
 * basis instead of a tower of extensions.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quartic extension field element.
 * @param[in] b				- the second quartic extension field element.
 */
void fb4_mul_dxd(fb4_t c, fb4_t a, fb4_t b);

/**
 * Multiplies two sparse quartic extension field elements.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quartic extension field element.
 * @param[in] b				- the second quartic extension field element.
 */
void fb4_mul_dxs(fb4_t c, fb4_t a, fb4_t b);

/**
 * Multiplies two sparse quartic extension field elements.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first quartic extension field element.
 * @param[in] b				- the second quartic extension field element.
 */
void fb4_mul_sxs(fb4_t c, fb4_t a, fb4_t b);

/**
 * Computes the square of a quartic extension field element. Computes
 * c = a * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quartic extension field element to square.
 */
void fb4_sqr(fb4_t c, fb4_t a);

/**
 * Inverts a quartic extension field element. Computes c = a^{-1}.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quartic extension field element to invert.
 */
void fb4_inv(fb2_t c, fb2_t a);

/**
 * Computes a power of a quartic extension field element. Computes c = a^b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quartic extension element to exponentiate.
 * @param[in] b				- the exponent.
 */
void fb4_exp(fb4_t c, fb4_t a, bn_t b);

/**
 * Computes the q-th power Frobenius map of a quartic extension field element.
 * Compute c = a^{2^m}.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the quartic extension element to exponentiate.
 */
void fb4_frb(fb4_t c, fb4_t a);

#endif /* !RELIC_FBX_H */
