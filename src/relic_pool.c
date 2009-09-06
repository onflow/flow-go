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
 * @file
 *
 * Implementation of the memory-management routines for the finite field
 * arithmetic modules.
 *
 * @version $Id: relic_pool.c 13 2009-04-16 02:24:55Z dfaranha $
 * @ingroup relic
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_dv.h"
#include "relic_bn.h"
#include "relic_pool.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if ALLOC == STATIC

/**
 * Type that represents a element of a pool of digit vectors.
 */
typedef struct {
	/** Indicates if this pool element is being used. */
	int state;
	/** The pool element. The extra digit stores the pool position. */
	align dig_t elem[DV_DIGS + 1];
} pool_t;

/** Indicates that the pool element is already used. */
#define POOL_USED	(1)
/** Indicates that the pool element is free. */
#define POOL_FREE	(0)
/** Indicates that the pool is empty. */
#define POOL_EMPTY (-1)

/**
 * The static pool of multiple precision integers.
 */
static pool_t pool[POOL_SIZE];

/**
 * The index of the next multiple precision integer not being used in the pool.
 */
static int next = 0;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

dig_t *pool_get(void) {
	int i, r;

	if (next == POOL_EMPTY)
		return NULL;

	/** Allocate a free element. */
	r = next;
	pool[r].state = POOL_USED;

	/* Search for a new free element. */
	for (i = (next + 1) % POOL_SIZE; i != next; i = (i + 1) % POOL_SIZE) {
		if (pool[i].state == POOL_FREE) {
			break;
		}
	}
	if (i == next) {
		next = POOL_EMPTY;
	} else {
		next = i;
	}

	pool[r].elem[DV_DIGS] = r;
	return (pool[r].elem);
}

void pool_put(dig_t *a) {
	int pos = a[DV_DIGS];
	pool[pos].state = POOL_FREE;
	next = pos;
}
#endif /* ALLOC == STATIC */
