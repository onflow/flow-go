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
 * @file
 *
 * Implementation of the low-level binary field bit shifting functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

.arch atmega128

#if FB_POLYN == 163

.global fb_rdcn_table0

.data

.text

.global fb_rdcn_low

.macro R0 r0, r1, r2
	ld		\r1, x

	inc		r27
	ld		\r0, x
	dec		r27

	swap	r26
	andi	r26, 0xF0
	lsl		r26
	mov		\r2, r26
.endm

.macro R r0, r1, r2, d1, d2
	ld		r23, x
	eor		\r1, r23

	inc		r27
	ld		r23, x
	dec		r27
	eor		\r0, r23

	swap	r26
	andi	r26, 0xF0
	lsl		r26
	mov		\r2, r26
.endm

.macro step1 d1,d2
	ldd		r26,y+\d1
	R r21, r22, r20
	ldd		r23,y+\d2
	eor		r23,r21
	std		z+\d2,r23
.endm

.macro step2 d1,d2
	ldd		r26,y+\d1
	R r22, r20, r21
	ldd		r23,y+\d2
	eor		r23,r22
	std		z+\d2,r23
.endm

.macro step3 d1,d2
	ldd		r26,y+\d1
	R r20, r21, r22
	ldd		r23,y+\d2
	eor		r23,r20
	std		z+\d2,r23
.endm

fb_rdcn_low:
	push	r28
	push	r29

	movw	r30,r24				; Copy c to z
	movw	r28,r22				; Copy a to y

	clr		r26
	ldi		r27, hi8(fb_rdcn_table0)

	ldd		r26, y+40
	R0 		r21, r22, r20
	ldd		r23, y+21
	eor		r23, r21
	mov		r25, r23			; We cannot write to c[21].

	ldd		r26, y+39
	R		r22, r20, r21
	ldd		r18, y + 20
	eor		r18, r22

	step3 38,19
	step1 37,18
	step2 36,17
	step3 35,16
	step1 34,15
	step2 33,14
	step3 32,13
	step1 31,12
	step2 30,11
	step3 29,10
	step1 28,9
	step2 27,8
	step3 26,7
	step1 25,6
	step2 24,5
	step3 23,4
	step1 22,3

	mov		r26, r25
	R 		r22, r20, r21
	ldd		r23, y + 2
	eor		r23, r22
	std		z + 2, r23

	ldd 	r24, y + 1
	eor 	r24, r20		; r24 = m[1]

	ldd		r25, y + 0
	eor		r25, r21		; r25 = m[0]

	mov		r20, r18
	lsr		r20
	swap	r20
	andi	r20, 0xC0	; r20 = (t>>3)<<6

	mov		r19, r20
	lsl		r19			; r19 = (t>>3)<<7

	mov		r21, r18
	andi	r21, 0xF8		; r21 = (t>>3)<<3

	mov		r22, r18
	lsr		r22
	lsr		r22
	lsr		r22			; r22 = (t >> 3)

	eor		r22, r19
	eor		r22, r20
	eor		r22, r21

	eor		r25, r22
	std		z + 0, r25

	mov		r25, r18
	swap	r25
	andi	r25, 0x0F
	mov		r23, r25
	lsr		r23
	eor		r23, r25

	eor		r24, r23
	std		z + 1, r24

	andi	r18, 0x07
	std		z + 20, r18

	pop		r29
	pop		r28
	ret
#endif
