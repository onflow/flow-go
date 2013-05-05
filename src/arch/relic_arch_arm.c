/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Implementation of architecture-dependent routines.
 *
 * @version $Id$
 * @ingroup arch
 */

#include "relic_util.h"

#ifdef __ARM_ARCH_7M__
	volatile unsigned int *DWT_CYCCNT = (unsigned int *)0xE0001004; //address of the register
	volatile unsigned int *DWT_CONTROL = (unsigned int *)0xE0001000; //address of the register
	volatile unsigned int *SCB_DEMCR = (unsigned int *)0xE000EDFC; //address of the register
#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void arch_init(void) {
#ifdef __ARM_ARCH_7M__
	*SCB_DEMCR = *SCB_DEMCR | 0x01000000;
	*DWT_CYCCNT = 0; // reset the counter
	*DWT_CONTROL = *DWT_CONTROL | 1 ; // enable the counter
#elif __ARM_ARCH_7A__
	asm("mcr p15, 0, %0, c9, c12, 0" :: "r"(17));
	asm("mcr p15, 0, %0, c9, c12, 1" :: "r"(0x8000000f));
	asm("mcr p15, 0, %0, c9, c12, 3" :: "r"(0x8000000f));
#endif
}

void arch_clean(void) {
}


unsigned long long arch_cycles(void) {
	unsigned int value = 0;
#ifdef __ARM_ARCH_7M__
	value = *DWT_CYCCNT;
#elif __ARM_ARCH_7A__
	asm("mcr p15, 0, %0, c9, c13, 0" : "=r"(value));  
#endif
	return value;
}
