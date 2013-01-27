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
 * Implementation of the low-level binary field squaring functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb_low.h"

#define RT		r24

#define RT2		r25

#define R0		r21

#define R1		r22

#define R2		r23

.arch atmega128

.text

.global fb_sqrl_low

.macro STEP i
	ldd		r18,z+\i
	mov		r28,r18
	andi	r28,0x0f
	ld		r19,y
	st		x+,r19
	swap	r18
	mov		r28,r18
	andi	r28,0x0f
	ld		r19,y
	st		x+,r19
.endm

.macro HALF_STEP i
	ldd		r18,z+\i
	mov		r28,r18
	andi	r28,0x0f
	ld		r19,y
	st		x+,r19
.endm

.macro SQRT_STEP i, j
	STEP \i
	.if \i < \j
		SQRT_STEP \i+1, \j
	.endif
.endm

fb_sqrl_low:
	push r28
	push r29

	movw	r30,r22				; Copy a to Z
	movw	r26,r24				; Copy c to X

	ldi		r28,lo8(fb_sqrt_table)
	ldi		r29,hi8(fb_sqrt_table)

	SQRT_STEP 0, FB_DIGS - 2

	STEP FB_DIGS - 1

	pop r29
	pop r28
	ret
