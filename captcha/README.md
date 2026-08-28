# Captcha

## Mode 模式

- [ ] String   字符串
- [X] Equation 算式 +,-,*

## Disturb 干扰项

- [x] Point   点
- [x] Line    直线
- [x] Arc     弧线
- [ ] Rotate  旋转
- [ ] Distort 扭曲

## 验证码字符字体

`MapleMonoNormalNL-Regular.ttf`

> [Maple Mono](https://github.com/subframe7536/maple-font)

```shell
fonttools subset ./MapleMonoNormalNL-Regular.ttf --text='+-*1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'```