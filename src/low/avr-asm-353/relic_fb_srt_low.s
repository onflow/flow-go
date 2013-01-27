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
 * Implementation of the low-level binary field square root.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb_low.h"

.arch atmega128

.text

.global fb_srti_low

.macro MACRO
	ld		r18, x+
	bst		r18, 0
	bld		r19, 0
	bst		r18, 2
	bld		r19, 1
	bst		r18, 4
	bld		r19, 2
	bst		r18, 6
	bld		r19, 3

	bst		r18, 1
	bld		r20, 0
	bst		r18, 3
	bld		r20, 1
	bst		r18, 5
	bld		r20, 2
	bst		r18, 7
	bld		r20, 3

	ld		r18, x+
	bst		r18, 0
	bld		r19, 4
	bst		r18, 2
	bld		r19, 5
	bst		r18, 4
	bld		r19, 6
	bst		r18, 6
	bld		r19, 7

	bst		r18, 1
	bld		r20, 4
	bst		r18, 3
	bld		r20, 5
	bst		r18, 5
	bld		r20, 6
	bst		r18, 7
	bld		r20, 7
.endm

fb_srti_low:
	push r28
	push r29

	movw	r30,r24				; Copy c to Z
	movw	r28,r22				; Copy t to Y
	movw	r26,r20				; Copy a to X

	clr		r22
	MACRO
	std		z+0, r19
	mov		r21, r20
	lsl		r21
	std		y+22,r21
	bst		r20, 7
	bld		r22, 0
	mov		r21, r20
	lsl		r21
	lsl		r21
	lsl		r21
	std		y+4, r21
	swap	r20
	andi	r20, 0x0F
	lsr		r20
	mov		r23, r20

	MACRO
	std		z+1, r19
	mov		r21, r20
	lsl		r21
	eor		r21, r22
	std		y+23,r21
	bst		r20, 7
	bld		r22, 0
	mov		r21, r20
	lsl		r21
	lsl		r21
	lsl		r21
	eor		r21, r23
	std		y+5, r21
	swap	r20
	andi	r20, 0x0F
	lsr		r20
	mov		r23, r20

	MACRO
	std		z+2, r19
	mov		r21, r20
	lsl		r21
	eor		r21, r22
	std		y+24,r21
	bst		r20, 7
	bld		r22, 0
	mov		r21, r20
	lsl		r21
	lsl		r21
	lsl		r21
	eor		r21, r23
	std		y+6, r21
	swap	r20
	andi	r20, 0x0F
	lsr		r20
	mov		r23, r20

	MACRO
	std		z+3, r19
	mov		r21, r20
	lsl		r21
	eor		r21, r22
	std		y+25,r21
	bst		r20, 7
	bld		r22, 0
	mov		r21, r20
	lsl		r21
	lsl		r21
	lsl		r21
	eor		r21, r23
	std		y+7, r21
	swap	r20
	andi	r20, 0x0F
	lsr		r20
	mov		r23, r20

	.irp i, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16
		MACRO
		ldd		r21, y+\i
		eor		r19, r21
		std		z+\i, r19
		mov		r21, r20
		lsl		r21
		eor		r21, r22
		std		y+22+\i,r21
		bst		r20, 7
		bld		r22, 0
		mov		r21, r20
		lsl		r21
		lsl		r21
		lsl		r21
		eor		r21, r23
		std		y+4+\i, r21
		swap	r20
		andi	r20, 0x0F
		lsr		r20
		mov		r23, r20
	.endr

	MACRO
	ldd		r21, y+17
	eor		r19, r21
	std		z+17, r19
	mov		r21, r20
	lsl		r21
	eor		r21, r22
	std		y+22+17,r21
	bst		r20, 7
	bld		r22, 0
	mov		r21, r20
	lsl		r21
	lsl		r21
	lsl		r21
	eor		r21, r23
	std		y+4+17, r21
	swap	r20
	andi	r20, 0x0F
	lsr		r20
	mov		r23, r20

	.irp i, 18, 19, 20, 21
		MACRO
		ldd		r21, y+\i
		eor		r19, r21
		std		z+\i, r19
		mov		r21, r20
		lsl		r21
		eor		r21, r22
		std		y+22+\i,r21
		bst		r20, 7
		bld		r22, 0
		mov		r21, r20
		lsl		r21
		lsl		r21
		lsl		r21
		eor		r21, r23
		ldd		r19, y+4+\i
		eor		r21, r19
		std		y+4+\i, r21
		swap	r20
		andi	r20, 0x0F
		lsr		r20
		mov		r23, r20
	.endr

	std		y+44, r22
	ldd		r19, y+26
	eor		r23, r19
	std		y+26, r23

	ld		r19, x+
	ldd		r21, y+22
	eor		r19, r21
	std		z+22, r19

	pop r29
	pop r28
	ret
