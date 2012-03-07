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
 * @defgroup arch Architecture-dependent utilities.
 */

/**
 * @file
 *
 * Interface of architecture-dependent utilities.
 *
 * @version $Id$
 * @ingroup arch
 */

#ifndef RELIC_ARCH_H
#define RELIC_ARCH_H

#include "relic_types.h"

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Return the number of elapsed cycles.
 */
unsigned long long arch_cycles(void);

/**
 * Performs architecture-dependent initialization.
 *
 * @return STS_OK if no error occurs, STS_ERR otherwise.
 */
int arch_init(void);

/**
 * Performs architecture-dependent finalization.
 *
 * @return STS_OK if no error occurs, STS_ERR otherwise.
 */
void arch_clean(void);

#endif /* !RELIC_ARCH_H */
