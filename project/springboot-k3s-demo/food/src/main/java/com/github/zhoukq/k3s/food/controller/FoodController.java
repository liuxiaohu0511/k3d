package com.github.zhoukq.k3s.food.controller;

import com.github.zhoukq.k3s.food.entiy.Product;
import org.apache.commons.rng.UniformRandomProvider;
import org.apache.commons.rng.simple.RandomSource;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.ArrayList;
import java.util.List;

@RestController
@RequestMapping("/")
public class FoodController {

    public static final Logger LOGGER = LoggerFactory.getLogger(FoodController.class);
    public static final UniformRandomProvider RNG = RandomSource.XO_RO_SHI_RO_128_PP.create();

    @RequestMapping("/products")
    public List<Product> products() {
        ArrayList<Product> list = new ArrayList<>();
        list.add(new Product("百事可乐", 3, RNG.nextInt(5, 500)));
        list.add(new Product("风干牛肉", 49.9, RNG.nextInt(3, 300)));
        list.add(new Product("去骨鸡爪", 38.9, RNG.nextInt(1, 300)));
        list.add(new Product("乐事薯片", 7.8, RNG.nextInt(1, 300)));
        list.add(new Product("恰恰瓜子", 12.5, RNG.nextInt(1, 300)));
        list.add(new Product("雀巢咖啡", 32.9, RNG.nextInt(1, 300)));
        return list;
    }


}
