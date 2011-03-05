/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Interface of the low-level prime field arithmetic module.
 *
 * @version $Id: relic_fp_low.h 459 2010-07-13 01:34:41Z dfaranha $
 * @ingroup fp
 */

#ifndef RELIC_PP_LOW_H
#define RELIC_PP_LOW_H

#include "relic_pp.h"

/**
 * Adds two digit vectors of the same size. Computes c = a + b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to multiply.
 * @param[in] b				- the second digit vector to multiply.
 */
void fp2_addn_low(fp2_t c, fp2_t a, fp2_t b);

/**
 * Subtracts two digit vectors of the same size. Computes c = a - b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to multiply.
 * @param[in] b				- the second digit vector to multiply.
 */
void fp2_subn_low(fp2_t c, fp2_t a, fp2_t b);

/**
 * Doubles a digit vectors. Computes c = a + a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to multiply.
 * @param[in] b				- the second digit vector to multiply.
 */
void fp2_dbln_low(fp2_t c, fp2_t a);

/**
 * Multiplies two digit vectors of the same size. Computes c = a * b.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first digit vector to multiply.
 * @param[in] b				- the second digit vector to multiply.
 */
void fp2_muln_low(fp2_t c, fp2_t a, fp2_t b);

/**
 * Square a digit vector. Computes c = a * a.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the digit vector to square.
 */
void fp2_sqrn_low(fp2_t c, fp2_t a);

#endif /* !RELIC_FP_LOW_H */
