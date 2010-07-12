/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009, 2010 RELIC Authors
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

.data

/*
 * This address finishes in 0x00.
 */

fb_sqrt_table:
.byte 0x00, 0x01, 0x04, 0x05, 0x10, 0x11, 0x14, 0x15, 0x40, 0x41, 0x44, 0x45, 0x50, 0x51, 0x54, 0x55

fb_sqrm_table0:
.byte 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 0x01, 0x06, 0x06, 0x06, 0x06, 0x07, 0x07, 0x07, 0x07

fb_sqrm_table1:
.byte 0x00, 0x19, 0x64, 0x7d, 0x92, 0x8b, 0xf6, 0xef, 0x48, 0x51, 0x2c, 0x35, 0xda, 0xc3, 0xbe, 0xa7

fb_sqrm_table2:
.byte 0x00, 0x20, 0x80, 0xa0, 0x00, 0x20, 0x80, 0xa0, 0x00, 0x20, 0x80, 0xa0, 0x00, 0x20, 0x80, 0xa0

/*
 * We must skip some bytes so that the next address in this segment stars on
 * a 246-byte aligned block
 */
.skip 256 - 4*16

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
