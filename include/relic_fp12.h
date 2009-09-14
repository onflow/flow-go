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
 * @defgroup fp12 Quadratic extension prime field arithmetic.
 */

/**
 * @file
 *
 * Interface of the quadratic extension prime field arithmetic functions.
 *
 * @version $Id$
 * @ingroup fp12
 */

#ifndef RELIC_FP12_H
#define RELIC_FP12_H

#include "relic_fp6.h"
#include "relic_types.h"

/**
 * Represents a quadratic extension prime field element.
 */
typedef fp6_t fp12_t[2];

/**
 * Allocate and initializes a quadratic extension prime field element.
 *
 * @param[out] A			- the new quadratic extension field element.
 */
#define fp12_new(A)															\
		fp6_new(A[0]); fp6_new(A[1]);										\

/**
 * Calls a function to clean and free a quadratic extension field element.
 *
 * @param[out] A			- the quadratic extension field element to free.
 */
#define fp12_free(A)														\
		fp6_free(A[0]); fp6_free(A[1]); 									\

/**
 * Copies the second argument to the first argument.
 *
 * @param[out] C			- the result.
 * @param[in] A				- the quadratic extension field element to copy.
 */
#define fp12_copy(C, A)														\
		fp6_copy(C[0], A[0]); fp6_copy(C[1], A[1]); 						\

/**
 * Negates a quadratic extension field element.
 *
 * @param[out] c			- the result.
 * @param[out] a			- the quadratic extension field element to negate.
 */
#define fp12_neg(C, A)														\
		fp6_neg(C[0], A[0]); fp6_neg(C[1], A[1]); 							\

/**
 * Assigns zero to a quadratic extension field element.
 *
 * @param[out] a			- the quadratic extension field element to zero.
 */
#define fp12_zero(A)														\
		fp6_zero(A[0]); fp6_zero(A[1]); 									\

/**
 * Tests if a quadratic extension field element is zero or not.
 *
 * @param[in] a				- the prime field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
#define fp12_is_zero(A)														\
		fp6_is_zero(A[0]) || fp6_is_zero(A[1]) 								\

/**
 * Assigns a random value to a quadratic extension field element.
 *
 * @param[out] a			- the prime field element to assign.
 */
#define fp12_rand(A)														\
		fp6_rand(A[0]); fp6_rand(A[1]);										\

/**
 * Prints a quadratic extension field element to standard output.
 *
 * @param[in] a				- the quadratic extension field element to print.
 */
#define fp12_print(A)														\
		fp6_print(A[0]); fp6_print(A[1]);									\

/**
 * Returns the result of a comparison between two quadratic extension field
 * elements
 *
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 * @return FP_LT if a < b, FP_EQ if a == b and FP_GT if a > b.
 */
#define fp12_cmp(A, B)														\
		((fp6_cmp(A[0], B[0]) == CMP_EQ) && (fp6_cmp(A[1], B[1]) == CMP_EQ)	\
		? CMP_EQ : CMP_NE)													\

#define fp12_set_dig(A, B)													\
		fp6_set_dig(A[0], B); fp6_zero(A[1]);								\

#define fp12_add_dig(C, A, B)												\
		fp6_add_dig(C[0], A[0], B); fp6_copy(C[1], A[1]);					\

#define fp12_sub_dig(C, A, B)												\
		fp6_sub_dig(C[0], A[0], B); fp6_copy(C[1], A[1]);					\

/**
 * Adds two quadratic extension field elements, that is, computes c = a + b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the first quadratic extension field element.
 * @param[in] b				- the second quadratic extension field element.
 */
#define fp12_add(C, A, B)													\
		fp6_add(C[0], A[0], B[0]); fp6_add(C[1], A[1], B[1]);				\

/**
 * Subtracts a prime field element from another, that is, computes
 * c = a - b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the quadratic extension prime field element.
 * @param[in] b				- the quadratic extension prime field element.
 */
#define fp12_sub(C, A, B)													\
		fp6_sub(C[0], A[0], B[0]); fp6_sub(C[1], A[1], B[1]);				\

/**
 * Multiples two prime field elements, that is, compute c = a * b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the quadratic extension prime field element.
 * @param[in] b				- the quadratic extension prime field element.
 */
void fp12_mul(fp12_t c, fp12_t a, fp12_t b);

/**
 * Computes the square of a prime field element, that is, computes
 * c = a * a.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the prime field element to square.
 */
void fp12_sqr(fp12_t c, fp12_t a);

/**
 *
 *
 */
void fp12_inv(fp12_t c, fp12_t a);

#endif /* !RELIC_FP12_H */
