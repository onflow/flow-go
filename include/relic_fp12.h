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
 * @defgroup fp12 Dodecic extension prime field arithmetic.
 */

/**
 * @file
 *
 * Interface of the dodecic extension prime field arithmetic functions.
 *
 * @version $Id$
 * @ingroup fp12
 */

#ifndef RELIC_FP12_H
#define RELIC_FP12_H

#include "relic_fp6.h"
#include "relic_types.h"

/**
 * Represents a dodecic extension prime field element.
 * 
 * An element a is represented as a[0] + a[1] * Z, where Z^2 = Y.
 */
typedef fp6_t fp12_t[2];

/**
 * Allocate and initializes a dodecic extension prime field element.
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
 * @param[out] c			- the result.
 * @param[out] a			- the dodecic extension field element to negate.
 */
#define fp12_neg(C, A)														\
		fp6_neg(C[0], A[0]); fp6_neg(C[1], A[1]); 							\

/**
 * Assigns zero to a dodecic extension field element.
 *
 * @param[out] a			- the dodecic extension field element to zero.
 */
#define fp12_zero(A)														\
		fp6_zero(A[0]); fp6_zero(A[1]); 									\

/**
 * Tests if a dodecic extension field element is zero or not.
 *
 * @param[in] a				- the prime field element to compare.
 * @return 1 if the argument is zero, 0 otherwise.
 */
#define fp12_is_zero(A)														\
		fp6_is_zero(A[0]) || fp6_is_zero(A[1]) 								\

/**
 * Assigns a random value to a dodecic extension field element.
 *
 * @param[out] a			- the dodecic extension prime field element to assign.
 */
#define fp12_rand(A)														\
		fp6_rand(A[0]); fp6_rand(A[1]);										\

/**
 * Prints a dodecic extension field element to standard output.
 *
 * @param[in] a				- the dodecic extension field element to print.
 */
#define fp12_print(A)														\
		fp6_print(A[0]); fp6_print(A[1]);									\

/**
 * Reads a dodecic extension field element from strings in a given radix.
 * The radix must be a power of 2 included in the interval [2, 64].
 *
 * @param[out] a			- the result.
 * @param[in] str000		- 
 * @param[in] str001		- 
 * @param[in] str010		- 
 * @param[in] str011		- 
 * @param[in] str020		- 
 * @param[in] str021		- 
 * @param[in] str100		- 
 * @param[in] str101		- 
 * @param[in] str110		- 
 * @param[in] str111		- 
 * @param[in] str120		- 
 * @param[in] str121		- 
 * @param[in] len 			- the maximum length of the strings.
 * @param[in] radix			- the radix.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
#define fp12_read(A, STR000, STR001, STR010, STR011, STR020, STR021, STR100, STR101, STR110, STR111, STR120, STR121, LEN, RADIX)\
		fp6_read(A[0], STR000, STR001, STR010, STR011, STR020, STR021, LEN, RADIX);\
		fp6_read(A[1], STR100, STR101, STR110, STR111, STR120, STR121, LEN, RADIX);\

/**
 * Writes a dodecic extension field element to strings in a given radix.
 * The radix must be a power of 2 included in the interval [2, 64].
 *
 * @param[out] str000		- 
 * @param[out] str001		- 
 * @param[out] str010		- 
 * @param[out] str011		- 
 * @param[out] str020		- 
 * @param[out] str021		- 
 * @param[out] str100		- 
 * @param[out] str101		- 
 * @param[out] str110		- 
 * @param[out] str111		- 
 * @param[out] str120		- 
 * @param[out] str121		- 
 * @param[in] len			- the buffer capacities.
 * @param[in] a				- the dodecic extension prime field element to write.
 * @param[in] radix			- the radix.
 * @throw ERR_BUFFER		- if the buffer capacity is insufficient.
 * @throw ERR_INVALID		- if the radix is invalid.
 */
#define fp12_write(STR000, STR001, STR010, STR011, STR020, STR021, STR100, STR101, STR110, STR111, STR120, STR121, LEN, A, RADIX)\
		fp6_write(STR000, STR001, STR010, STR011, STR020, STR021, LEN, A[0], RADIX);\
		fp6_write(STR100, STR101, STR110, STR111, STR120, STR121, LEN, A[1], RADIX);\

/**
 * Returns the result of a comparison between two dodecic extension field
 * elements
 *
 * @param[in] a				- the first dodecic extension field element.
 * @param[in] b				- the second dodecic extension field element.
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
 * Adds two dodecic extension field elements, that is, computes c = a + b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the first dodecic extension field element.
 * @param[in] b				- the second dodecic extension field element.
 */
#define fp12_add(C, A, B)													\
		fp6_add(C[0], A[0], B[0]); fp6_add(C[1], A[1], B[1]);				\

/**
 * Subtracts a dodecic extension field element from another, that is, computes
 * c = a - b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the first dodecic extension prime field element.
 * @param[in] b				- the second dodecic extension prime field element.
 */
#define fp12_sub(C, A, B)													\
		fp6_sub(C[0], A[0], B[0]); fp6_sub(C[1], A[1], B[1]);				\

/**
 * Multiples two dodecic extension field elements, that is, compute c = a * b.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the first dodecic extension prime field element.
 * @param[in] b				- the second dodecic extension prime field element.
 */
void fp12_mul(fp12_t c, fp12_t a, fp12_t b);

/**
 * Computes the square of a dodecic extension field element, that is, computes
 * c = a * a.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the dodecic extension prime field element to square.
 */
void fp12_sqr(fp12_t c, fp12_t a);

/**
 * Inverts a dodecic extension field element. Computes c = a^(-1).
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the dodecic extension prime field element to invert.
 */
void fp12_inv(fp12_t c, fp12_t a);

/**
 * Multiples a dodecic extension field element by a sparse element.
 *
 * The sparse element must have only the a[0][0], a[1][0] and a[1][1] elements
 * not zero.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- a dodecic extension prime field element.
 * @param[in] b				- a sparse dodecic extension prime field element.
 */
void fp12_mul_sparse(fp12_t c, fp12_t a, fp12_t b);

/**
 * Computes the square of a unitary dodecic extension field element.
 * 
 * A unitary element is one previously raised to the (p^6 - 1)th power.
 *
 * @param[out] c			- the destination.
 * @param[in] a				- the unitary dodecic extension prime field element to square.
 */
void fp12_sqr_uni(fp12_t c, fp12_t a);

/**
 * Computes the Frobenius endomorphism of a unitary dodecic extension field element,
 * that is, Frob(a) = a^p.
 * 
 * @param[out] c			- the destination.
 * @param[in] a				- a dodecic extension prime field element.
 * @param[in] b				- constant used in Frobenius, Z^p. 
 */
void fp12_frob(fp12_t c, fp12_t a, fp12_t b);

/**
 * Computes the conjugate of a dodecic extension field element.
 * 
 * For unitary elements, this is equivalent to computing the inverse.
 * A unitary element is one previously raised to the (p^6 - 1)th power.
 * 
 * @param[out] c			- the destination.
 * @param[in] a				- the unitary dodecic extension prime field element to conjugate.
 */
void fp12_conj(fp12_t c, fp12_t a);

/**
 * Computes the b-th power of a unitary dodecid extension field element.
 * 
 * A unitary element is one previously raised to the (p^6 - 1)th power.
 * 
 * @param[out] c			- the destination.
 * @param[in] a				- the unitary dodecic extension prime field element.
 * @param[in] b				- the power. 
 */
void fp12_exp_basic_uni(fp12_t c, fp12_t a, bn_t b);

#endif /* !RELIC_FP12_H */
