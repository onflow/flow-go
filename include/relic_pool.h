/*
 * Copyright 2007-2009 RELIC Project
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
 * @file
 *
 * Interface of the memory-management routines for the finite field
 * arithmetic modules.
 *
 * @version $Id: relic_pool.h 3 2009-04-08 01:05:51Z dfaranha $
 * @ingroup relic
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_conf.h"
#include "relic_util.h"

#if ALLOC == STATIC

/**
 * The size of the static pool of digit vectors.
 */
#ifndef POOL_SIZE
#define POOL_SIZE	(50 * MAX(TESTS, BENCH * BENCH))
#endif

/**
 * Gets a new element from the static pool.
 *
 * @returns the address of a free element in the static pool.
 */
dig_t *pool_get(void);

/**
 * Restores an element to the static pool.
 *
 * @param[in] a			- the address to free.
 */
void pool_put(dig_t *a);

#endif /* ALLOC == STATIC */
