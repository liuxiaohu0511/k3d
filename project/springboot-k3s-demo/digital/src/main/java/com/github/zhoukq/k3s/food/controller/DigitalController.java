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
public class DigitalController {

    public static final Logger LOGGER = LoggerFactory.getLogger(DigitalController.class);
    public static final UniformRandomProvider RNG = RandomSource.XO_RO_SHI_RO_128_PP.create();

    @RequestMapping("/products")
    public List<Product> products() {
        ArrayList<Product> list = new ArrayList<>();
        list.add(new Product("iPhone 15", 5999, RNG.nextInt(5, 500)));
        list.add(new Product("Xiaomi 14", 3799, RNG.nextInt(3, 300)));
        list.add(new Product("Huawei Mate 60 Pro", 7999, RNG.nextInt(1, 300)));
        return list;
    }

}
